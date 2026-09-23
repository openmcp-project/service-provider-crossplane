/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrlutils "github.com/openmcp-project/controller-utils/pkg/controller"
	commonapi "github.com/openmcp-project/openmcp-operator/api/common"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
)

func cond(condType string, status metav1.ConditionStatus, reason string) metav1.Condition {
	return metav1.Condition{Type: condType, Status: status, Reason: reason}
}

func Test_computePhase(t *testing.T) {
	cases := []struct {
		desc       string
		conditions []metav1.Condition
		deleting   bool
		want       string
	}{
		{
			desc: "no conditions → Progressing",
			want: commonapi.StatusPhaseProgressing,
		},
		{
			desc:       "all True → Ready",
			conditions: []metav1.Condition{cond("CrossplaneReady", metav1.ConditionTrue, "Healthy")},
			want:       commonapi.StatusPhaseReady,
		},
		{
			desc: "False/Failing condition → Progressing",
			conditions: []metav1.Condition{
				cond("CrossplaneReady", metav1.ConditionTrue, "Healthy"),
				cond("ProviderBtpReady", metav1.ConditionFalse, "Unhealthy"),
			},
			want: commonapi.StatusPhaseProgressing,
		},
		{
			desc: "False/Uninstalled condition only → Ready (not Progressing)",
			conditions: []metav1.Condition{
				cond("CrossplaneReady", metav1.ConditionTrue, "Healthy"),
				cond("ProviderCloudfoundryReady", metav1.ConditionFalse, "Uninstalled"),
			},
			want: commonapi.StatusPhaseReady,
		},
		{
			desc: "multiple Uninstalled conditions alongside healthy ones → Ready",
			conditions: []metav1.Condition{
				cond("CrossplaneReady", metav1.ConditionTrue, "Healthy"),
				cond("ProviderCloudfoundryReady", metav1.ConditionFalse, "Uninstalled"),
				cond("DeploymentRuntimeConfigDefaultReady", metav1.ConditionFalse, "Uninstalled"),
			},
			want: commonapi.StatusPhaseReady,
		},
		{
			desc: "Uninstalled and a real failure → Progressing",
			conditions: []metav1.Condition{
				cond("CrossplaneReady", metav1.ConditionTrue, "Healthy"),
				cond("ProviderCloudfoundryReady", metav1.ConditionFalse, "Uninstalled"),
				cond("ProviderBtpReady", metav1.ConditionFalse, "Unhealthy"),
			},
			want: commonapi.StatusPhaseProgressing,
		},
		{
			desc:     "deleting object → Terminating regardless of conditions",
			deleting: true,
			conditions: []metav1.Condition{
				cond("CrossplaneReady", metav1.ConditionTrue, "Healthy"),
			},
			want: commonapi.StatusPhaseTerminating,
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			obj := &v1alpha1.Crossplane{}
			obj.Status.Conditions = tc.conditions
			if tc.deleting {
				now := metav1.Now()
				obj.DeletionTimestamp = &now
			}
			got, err := computePhase(obj, ctrlutils.ReconcileResult[*v1alpha1.Crossplane]{})
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_smartRequeueConditional(t *testing.T) {
	cases := []struct {
		desc       string
		object     *v1alpha1.Crossplane
		want       ctrlutils.SmartRequeueAction
	}{
		{
			desc:   "nil object → NoRequeue",
			object: nil,
			want:   ctrlutils.SR_NO_REQUEUE,
		},
		{
			desc: "all True → Backoff",
			object: &v1alpha1.Crossplane{
				Status: v1alpha1.CrossplaneStatus{
					Conditions: []metav1.Condition{
						cond("CrossplaneReady", metav1.ConditionTrue, "Healthy"),
					},
				},
			},
			want: ctrlutils.SR_BACKOFF,
		},
		{
			desc: "False/Failing → Reset",
			object: &v1alpha1.Crossplane{
				Status: v1alpha1.CrossplaneStatus{
					Conditions: []metav1.Condition{
						cond("ProviderBtpReady", metav1.ConditionFalse, "Unhealthy"),
					},
				},
			},
			want: ctrlutils.SR_RESET,
		},
		{
			desc: "False/Uninstalled only → Backoff (not Reset)",
			object: &v1alpha1.Crossplane{
				Status: v1alpha1.CrossplaneStatus{
					Conditions: []metav1.Condition{
						cond("CrossplaneReady", metav1.ConditionTrue, "Healthy"),
						cond("ProviderCloudfoundryReady", metav1.ConditionFalse, "Uninstalled"),
						cond("DeploymentRuntimeConfigDefaultReady", metav1.ConditionFalse, "Uninstalled"),
					},
				},
			},
			want: ctrlutils.SR_BACKOFF,
		},
		{
			desc: "Uninstalled and real failure → Reset",
			object: &v1alpha1.Crossplane{
				Status: v1alpha1.CrossplaneStatus{
					Conditions: []metav1.Condition{
						cond("CrossplaneReady", metav1.ConditionTrue, "Healthy"),
						cond("ProviderCloudfoundryReady", metav1.ConditionFalse, "Uninstalled"),
						cond("ProviderBtpReady", metav1.ConditionFalse, "Unhealthy"),
					},
				},
			},
			want: ctrlutils.SR_RESET,
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			rr := ctrlutils.ReconcileResult[*v1alpha1.Crossplane]{Object: tc.object}
			assert.Equal(t, tc.want, smartRequeueConditional(rr))
		})
	}
}
