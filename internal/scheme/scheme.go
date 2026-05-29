package scheme

import (
	crossplanev1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	crossplanev1beta1 "github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	clustersv1alpha1 "github.com/openmcp-project/openmcp-operator/api/clusters/v1alpha1"
	providersv1alpha1 "github.com/openmcp-project/openmcp-operator/api/provider/v1alpha1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
)

var (
	// Platform is the scheme when communicating with the platform cluster.
	Platform = runtime.NewScheme()

	// Onboarding is the scheme when communicating with the onboarding cluster.
	Onboarding = runtime.NewScheme()

	// MCP is the scheme for communicating with the ManagedControlPlane (MCP) cluster.
	MCP = runtime.NewScheme()
)

// initPlatform registers the platform schemes.
func initPlatform() {
	utilruntime.Must(clientgoscheme.AddToScheme(Platform))
	utilruntime.Must(v1alpha1.AddToScheme(Platform))
	utilruntime.Must(clustersv1alpha1.AddToScheme(Platform))
	utilruntime.Must(apiextv1.AddToScheme(Platform))
	utilruntime.Must(providersv1alpha1.AddToScheme(Platform))

	// Flux CD
	utilruntime.Must(helmv2.AddToScheme(Platform))
	utilruntime.Must(kustomizev1.AddToScheme(Platform))
	utilruntime.Must(sourcev1.AddToScheme(Platform))
}

// initOnboarding registers the onboarding schemes.
func initOnboarding() {
	utilruntime.Must(clientgoscheme.AddToScheme(Onboarding))
	utilruntime.Must(apiextv1.AddToScheme(Onboarding))
	utilruntime.Must(v1alpha1.AddToScheme(Onboarding))
}

// initMCP registers the MCP schemes.
func initMCP() {
	utilruntime.Must(clientgoscheme.AddToScheme(MCP))
	utilruntime.Must(apiextv1.AddToScheme(Onboarding))

	// Crossplane
	utilruntime.Must(crossplanev1.AddToScheme(MCP))
	utilruntime.Must(crossplanev1beta1.AddToScheme(MCP))
}

func init() {
	initPlatform()
	initOnboarding()
	initMCP()
}
