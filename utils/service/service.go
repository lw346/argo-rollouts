package service

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
)

func GetRolloutSelectorLabel(svc *corev1.Service) (string, bool) {
	if svc == nil {
		return "", false
	}
	if svc.Spec.Selector == nil {
		return "", false
	}
	currentSelectorValue, ok := svc.Spec.Selector[v1alpha1.DefaultRolloutUniqueLabelKey]
	return currentSelectorValue, ok
}

// GetRolloutServiceKeys returns services keys (namespace/serviceName) which are referenced by specified rollout
func GetRolloutServiceKeys(rollout *v1alpha1.Rollout) []string {
	servicesSet := make(map[string]bool)
	if rollout.Spec.Strategy.BlueGreenStrategy != nil {
		if rollout.Spec.Strategy.BlueGreenStrategy.ActiveService != "" {
			servicesSet[fmt.Sprintf("%s/%s", rollout.Namespace, rollout.Spec.Strategy.BlueGreenStrategy.ActiveService)] = true
		}
		if rollout.Spec.Strategy.BlueGreenStrategy.PreviewService != "" {
			servicesSet[fmt.Sprintf("%s/%s", rollout.Namespace, rollout.Spec.Strategy.BlueGreenStrategy.PreviewService)] = true
		}
	} else if rollout.Spec.Strategy.CanaryStrategy != nil {
		if rollout.Spec.Strategy.CanaryStrategy.CanaryService != "" {
			servicesSet[fmt.Sprintf("%s/%s", rollout.Namespace, rollout.Spec.Strategy.CanaryStrategy.CanaryService)] = true
		}
	}
	var services []string
	for svc := range servicesSet {
		services = append(services, svc)
	}
	return services
}
