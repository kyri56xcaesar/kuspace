package uspace

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"sort"
	"strings"
	"testing"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

// flakyStorage fails storage calls at random. Half the failures happen
// before the call takes effect, half after it (the object was written or
// removed, but the answer got lost) - the two ways a crash or a network
// error can cut an operation.
type flakyStorage struct {
	*memStorage
	r    *rand.Rand
	rate float64 // chance of a failure per call
}

var errInjected = fmt.Errorf("injected failure (%w)", ut.ErrUnavailable)

func (f *flakyStorage) fault(apply func() error) error {
	if f.r.Float64() >= f.rate {
		return apply()
	}
	if f.r.IntN(2) == 0 {
		return errInjected // before: nothing happened
	}
	_ = apply()

	return errInjected // after: it happened, the caller hears otherwise
}

func (f *flakyStorage) Insert(ctx context.Context, t any) error {
	return f.fault(func() error { return f.memStorage.Insert(ctx, t) })
}

func (f *flakyStorage) Remove(ctx context.Context, t any) error {
	return f.fault(func() error { return f.memStorage.Remove(ctx, t) })
}

func (f *flakyStorage) Copy(ctx context.Context, s, d any) error {
	return f.fault(func() error { return f.memStorage.Copy(ctx, s, d) })
}

// TestNoRecordWithoutItsObject: whatever fails in between, every file uspace
// lists has its object. (Orphan objects - bytes nobody lists - are allowed;
// they are invisible and collectable.) This is the write-ordering rule:
// object first/record last on create, record first/object last on delete.
func TestNoRecordWithoutItsObject(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		t.Run(fmt.Sprint("seed", seed), func(t *testing.T) {
			a := newAPIHarness(t, 1000) // quotas out of the way
			store := &flakyStorage{memStorage: &memStorage{objects: map[string][]byte{}}, r: rand.New(rand.NewPCG(seed, 7)), rate: 0.3}
			a.srv.storage = store
			r := rand.New(rand.NewPCG(seed, 13))
			const who = "1001:1001"
			volumes := []string{"vol1", "other"}
			name := func() string { return fmt.Sprintf("f%d.txt", r.IntN(8)) }
			for step := range 250 {
				vol := volumes[r.IntN(2)]
				var desc string
				switch r.IntN(4) {
				case 0:
					n := name()
					desc = "upload " + vol + "/" + n
					a.upload(vol, n, strings.Repeat("x", r.IntN(50)+1), who)
				case 1:
					n := name()
					desc = "rm " + vol + "/" + n
					a.do(http.MethodDelete, "/api/v1/resource/rm", vol+":/"+n, who, nil, "")
				case 2:
					n, m, to := name(), name(), volumes[r.IntN(2)]
					desc = "cp " + vol + "/" + n + " -> " + to + "/" + m
					a.do(http.MethodPost, "/api/v1/resource/cp?dest="+to+"/"+m, vol+":/"+n, who, nil, "")
				default:
					n, m, to := name(), name(), volumes[r.IntN(2)]
					desc = "mv " + vol + "/" + n + " -> " + to + "/" + m
					a.do(http.MethodPatch, "/api/v1/resource/mv?dest="+to+"/"+m, vol+":/"+n, who, nil, "")
				}
				if missing := recordsWithoutObjects(t, a, store.memStorage); len(missing) > 0 {
					t.Fatalf("step %d (%s): listed without their object: %v", step, desc, missing)
				}
			}
		})
	}
}

func recordsWithoutObjects(t *testing.T, a *apiHarness, store *memStorage) []string {
	t.Helper()
	res, err := a.srv.fsl.SelectObjects(t.Context(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	var missing []string
	for _, r := range res.([]ut.Resource) {
		if _, ok := store.objects[key(r.Vname, r.Name)]; !ok {
			missing = append(missing, r.Vname+r.Name)
		}
	}
	sort.Strings(missing)

	return missing
}
