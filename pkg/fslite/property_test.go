package fslite

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"testing"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

// A model of what the store should hold: files per volume with owner and
// size, and the limits. Random operations run against both; after every
// step the store must agree with the model.
type quotaModel struct {
	files      map[string]map[string]modelFile // volume -> name -> file
	capacity   map[string]int64                // bytes, 0 = unlimited
	groupQuota map[string]int64                // group volumes: bytes
	userQuota  map[[2]string]int64             // [volume, uid] -> bytes (set explicitly)
	defQuota   int64
}

type modelFile struct {
	uid  int64
	size int64
}

func (m *quotaModel) used(volume string, uid int64, byOwner bool) int64 {
	var sum int64
	for _, f := range m.files[volume] {
		if !byOwner || f.uid == uid {
			sum += f.size
		}
	}

	return sum
}

// fits: what an enforced insert must decide.
func (m *quotaModel) fits(volume string, uid, size int64) bool {
	if c := m.capacity[volume]; c > 0 && m.used(volume, 0, false)+size > c {
		return false
	}
	if uid == 0 {
		return true
	}
	if q, shared := m.groupQuota[volume]; shared {
		return q == 0 || m.used(volume, 0, false)+size <= q
	}
	q, ok := m.userQuota[[2]string{volume, strconv.FormatInt(uid, 10)}]
	if !ok {
		q = m.defQuota
	}

	return q == 0 || m.used(volume, uid, true)+size <= q
}

const mb = int64(1_000_000)

// TestQuotaAccountingProperty: accepted and refused inserts match the
// model exactly, and the reported usage always equals the files' sizes.
func TestQuotaAccountingProperty(t *testing.T) {
	for seed := uint64(1); seed <= 8; seed++ {
		t.Run(fmt.Sprint("seed", seed), func(t *testing.T) {
			r := rand.New(rand.NewPCG(seed, 99))
			fsl := newTestFsl(t)
			m := &quotaModel{
				files:      map[string]map[string]modelFile{"data": {}, "small": {}, "team": {}},
				capacity:   map[string]int64{"data": 0, "small": 300 * mb, "team": 0},
				groupQuota: map[string]int64{"team": 400 * mb},
				userQuota:  map[[2]string]int64{},
				defQuota:   200 * mb,
			}
			for v, c := range m.capacity {
				mustVolume(t, fsl, v, float64(c)/1e9)
			}
			if _, err := fsl.AssignGroupVolume(ctx, "team", 500, 0.4); err != nil {
				t.Fatal(err)
			}
			volumes := []string{"data", "small", "team"}
			uids := []int64{0, 1001, 1002, 1003}
			for step := range 300 {
				vol := volumes[r.IntN(len(volumes))]
				uid := uids[r.IntN(len(uids))]
				switch op := r.IntN(10); {
				case op < 6: // insert
					name := fmt.Sprintf("f%d", r.IntN(40))
					size := r.Int64N(120*mb) + 1
					_, taken := m.files[vol]["/"+name]
					err := fsl.InsertResource(ctx, file(name, vol, uid, size), float64(m.defQuota)/1e9, true)
					switch {
					case taken:
						if !errors.Is(err, ErrResourceExists) {
							t.Fatalf("step %d: insert over %s/%s: %v", step, vol, name, err)
						}
					case m.fits(vol, uid, size):
						if err != nil {
							t.Fatalf("step %d: model accepts %d bytes for uid %d on %s, store refused: %v", step, size, uid, vol, err)
						}
						m.files[vol]["/"+name] = modelFile{uid, size}
					default:
						if !errors.Is(err, ErrQuotaExceeded) && !errors.Is(err, ErrVolumeFull) {
							t.Fatalf("step %d: model refuses %d bytes for uid %d on %s, store said %v", step, size, uid, vol, err)
						}
					}
				case op < 8: // delete
					for name := range m.files[vol] {
						if err := fsl.Remove(ctx, ut.Resource{Name: name, Vname: vol}); err != nil {
							t.Fatalf("step %d: remove %s: %v", step, name, err)
						}
						delete(m.files[vol], name)

						break
					}
				case op < 9: // a job overwrote a file: new size, recorded as is
					for name, f := range m.files[vol] {
						f.size = r.Int64N(100 * mb)
						if err := fsl.SetObjectSize(ctx, name, vol, f.size); err != nil {
							t.Fatalf("step %d: set size: %v", step, err)
						}
						m.files[vol][name] = f

						break
					}
				default: // an admin sets a personal quota
					if vol == "team" || uid == 0 {
						continue
					}
					q := r.Int64N(300 * mb)
					if _, err := fsl.SetUserQuota(ctx, vol, uid, float64(q)/1e9); err != nil {
						t.Fatalf("step %d: set quota: %v", step, err)
					}
					m.userQuota[[2]string{vol, strconv.FormatInt(uid, 10)}] = q
				}
				checkUsage(t, fsl, m, step)
			}
		})
	}
}

// checkUsage: every usage the store reports equals the model's file sizes.
func checkUsage(t *testing.T, fsl *FsLite, m *quotaModel, step int) {
	t.Helper()
	near := func(gb float64, bytes int64) bool { return math.Abs(gb*1e9-float64(bytes)) < 1 }
	vols, err := fsl.SelectVolumes(ctx, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vols.([]ut.Volume) {
		if _, modeled := m.files[v.Name]; modeled && !near(v.Usage, m.used(v.Name, 0, false)) {
			t.Fatalf("step %d: volume %s usage %v GB, files hold %d bytes", step, v.Name, v.Usage, m.used(v.Name, 0, false))
		}
	}
	uvs, err := fsl.UserVolumes(ctx, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, uv := range uvs {
		if want := m.used(uv.Vname, uv.UID, true); !near(uv.Usage, want) {
			t.Fatalf("step %d: uid %d on %s: usage %v GB, owns %d bytes", step, uv.UID, uv.Vname, uv.Usage, want)
		}
	}
	gv, err := fsl.GroupVolume(ctx, "team")
	if err != nil || !near(gv.Usage, m.used("team", 0, false)) {
		t.Fatalf("step %d: group usage %v GB, files hold %d bytes (%v)", step, gv.Usage, m.used("team", 0, false), err)
	}
}
