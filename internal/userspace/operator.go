package userspace

import (
	"context"
	"log"
	/* CRD */
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	/* group registering */
	"k8s.io/apimachinery/pkg/runtime/schema"

	/* reconciler */
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	/* controller */
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Userspace defines the schema for the Userspace CRD
type Userspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserspaceSpec   `json:"spec,omitempty"`
	Status UserspaceStatus `json:"status,omitempty"`
}

type UserspaceSpec struct {
	PVName      string `json:"pvName"`
	StorageSize string `json:"storageSize"`
	Owner       string `json:"owner"`
}

type UserspaceStatus struct {
	PVStatus string `json:"pvStatus"`
}

/* ***********************************************************************************8 */
// UserspaceList for listing multiple resources
type UserspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Userspace `json:"items"`
}

/* group registering */
var (
	GroupVersion = schema.GroupVersion{Group: "example.com", Version: "v1"}
)

func AddToScheme(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion, &Userspace{}, &UserspaceList{})
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}

/* ***********************************************************************************8 */

func (u *Userspace) DeepCopyObject() runtime.Object {
	return u
}

func (ul *UserspaceList) DeepCopyObject() runtime.Object {
	return ul
}

/* ***********************************************************************************8 */

type UserspaceReconciler struct {
	client.Client
}

func (r *UserspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Printf("Reconciling Userspace %s", req.NamespacedName)

	// Fetch the Userspace CR
	userspace := &Userspace{}
	if err := r.Client.Get(ctx, req.NamespacedName, userspace); err != nil {
		// Ignore not-found errors
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Reconciliation logic: Ensure PV exists and matches spec
	err := r.ensurePersistentVolume(userspace)
	if err != nil {
		log.Printf("Failed to ensure PV for Userspace %s: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *UserspaceReconciler) ensurePersistentVolume(userspace *Userspace) error {
	// Logic to create or update a PersistentVolume
	return nil
}

/* ***********************************************************************************8 */

func (r *UserspaceReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&Userspace{}). // Watch Userspace CR
		Complete(r)
}

/* ***********************************************************************************8 */
