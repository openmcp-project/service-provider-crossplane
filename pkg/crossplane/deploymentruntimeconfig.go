package crossplane

import (
	"path"

	crossplanev1beta1 "github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// CABundleConfigMapName is the name of the ConfigMap containing a custom CA bundle.
	CABundleConfigMapName = "custom-ca-bundle"

	caBundleVolumeName = "custom-ca-bundle"
	caBundleMountDir   = "/etc/custom-ca"

	// certFileEnv is the environment variable which identifies where to locate
	// the SSL certificate file. If set this overrides the system default.
	certFileEnv = "SSL_CERT_FILE"

	// certDirEnv is the environment variable which identifies which directory
	// to check for SSL certificate files. If set this overrides the system default.
	// It is a colon separated list of directories.
	// See https://www.openssl.org/docs/man1.0.2/man1/c_rehash.html.
	certDirEnv = "SSL_CERT_DIR"
)

// GetDeploymentTemplateForCABundleRef creates a DeploymentTemplate that configures
// a Crossplane package runtime container to use a custom CA bundle from a ConfigMap.
// It mounts the ConfigMap as a volume and sets SSL_CERT_FILE and SSL_CERT_DIR
// environment variables to enable the runtime to trust certificates signed by the
// custom CA.
func GetDeploymentTemplateForCABundleRef(ref *corev1.ConfigMapKeySelector) *crossplanev1beta1.DeploymentTemplate {
	return &crossplanev1beta1.DeploymentTemplate{
		Spec: &v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "package-runtime",
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: caBundleMountDir,
									Name:      caBundleVolumeName,
									ReadOnly:  true,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  certFileEnv,
									Value: path.Join(caBundleMountDir, ref.Key),
								},
								{
									Name:  certDirEnv,
									Value: caBundleMountDir,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: caBundleVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: ref.Name,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
