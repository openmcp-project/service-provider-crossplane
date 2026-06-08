package crossplane

import (
	"strings"

	crossplanev1beta1 "github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// CABundleConfigMapName is the name of the ConfigMap containing a custom CA bundle.
	CABundleConfigMapName = "custom-ca-bundle"

	// caBundleVolumeName is the name of the custom ca volume and volume mount
	caBundleVolumeName = "custom-ca-bundle"

	// caBundleMountDir is the path the ca bundle will be mounted into
	caBundleMountDir = "/etc/open-control-plane/custom-ca"

	// certDirEnv is the environment variable which identifies which directory
	// to check for SSL certificate files. It is a colon separated list of directories.
	// See https://www.openssl.org/docs/man1.0.2/man1/c_rehash.html.
	certDirEnv = "SSL_CERT_DIR"
)

// certDirectories contains a list of places where the default system certs are stored in addition to caBundleMountDir
// from x509 go lib (https://github.com/golang/go/blob/015343854b5d9e2829481df30dbcae2ca6682d25/src/crypto/x509/root_linux.go)
var certDirectories = []string{
	"/etc/ssl/certs",
	"/etc/pki/tls/certs",
}

// GetDeploymentTemplateForCABundleRef creates a DeploymentTemplate that configures
// a Crossplane package runtime container to use a custom CA bundle from a ConfigMap.
// It mounts the ConfigMap as a volume and sets SSL_CERT_DIR environment variable to
// enable the runtime to trust certificates signed by the custom CA in addition to public ones.
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
									Name:  certDirEnv,
									Value: strings.Join(append(certDirectories, caBundleMountDir), ":"),
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
									Items: []corev1.KeyToPath{
										{
											Key:  ref.Key,
											Path: ref.Key,
										},
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
