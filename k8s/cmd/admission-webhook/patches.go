package main

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	nsmcorednsenv "github.com/networkservicemesh/networkservicemesh/k8s/cmd/nsm-coredns/env"

	"github.com/networkservicemesh/networkservicemesh/sdk/client"
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func createDNSPatch(tuple *podSpecAndMeta, annotationValue string) (patch []patchOperation) {
	// TODO: now order of containers is important since nsmdp assign proper workspace only to the first container
	patch = append(patch, addContainer(tuple.spec,
		[]corev1.Container{
			{
				Name:            "nsm-dns-monitor",
				Image:           fmt.Sprintf("%s/%s:%s", getRepo(), "nsm-monitor", getTag()),
				ImagePullPolicy: corev1.PullIfNotPresent,
				Env: []corev1.EnvVar{
					{
						Name:  "MONITOR_DNS_CONFIGS",
						Value: "true",
					},
					{
						Name:  nsmcorednsenv.UpdateAPIClientSock.Name(),
						Value: "/etc/coredns/client.sock"},
					{
						Name:  client.AnnotationEnv,
						Value: annotationValue,
					},
				},
				VolumeMounts: []corev1.VolumeMount{{
					ReadOnly:  false,
					Name:      "nsm-coredns-volume",
					MountPath: "/etc/coredns",
				}},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"networkservicemesh.io/socket": resource.MustParse("1"),
					},
				},
			},
		})...)
	patch = append(patch, addContainer(tuple.spec,
		[]corev1.Container{
			{
				Name:            "nsm-coredns",
				Image:           fmt.Sprintf("%s/%s:%s", getRepo(), "nsm-coredns", getTag()),
				ImagePullPolicy: corev1.PullIfNotPresent,
				Args:            []string{"-conf", "/etc/coredns/Corefile"},
				VolumeMounts: []corev1.VolumeMount{{
					ReadOnly:  false,
					Name:      "nsm-coredns-volume",
					MountPath: "/etc/coredns",
				}},
				Env: []corev1.EnvVar{
					{
						Name:  nsmcorednsenv.UseUpdateAPIEnv.Name(),
						Value: "true",
					},
					{
						Name:  nsmcorednsenv.UpdateAPIClientSock.Name(),
						Value: "/etc/coredns/client.sock",
					},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"networkservicemesh.io/socket": resource.MustParse("1"),
					},
				},
			},
		})...)
	permission := int32(420)
	patch = append(patch, addVolume(tuple.spec,
		[]corev1.Volume{{
			Name: "nsm-coredns-volume",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium:    corev1.StorageMediumDefault,
					SizeLimit: nil,
				},
			},
		},
		{
			Name: "nsm-podinfo-volume",
			VolumeSource: corev1.VolumeSource{
				DownwardAPI: &corev1.DownwardAPIVolumeSource{
					DefaultMode: &permission,
					Items: []corev1.DownwardAPIVolumeFile{{
						Path: "uid",
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.uid",
						},
					}},
				},
			},
		}})...)
	return patch
}

func createNsmInitContainerPatch(annotationValue string) []patchOperation {
	var patch []patchOperation

	envVals := []corev1.EnvVar{{
		Name:  client.AnnotationEnv,
		Value: annotationValue,
	}}
	jaegerHost := getJaegerHost()
	if jaegerHost != "" {
		envVals = append(envVals,
			corev1.EnvVar{
				Name:  jaegerHostEnv,
				Value: jaegerHost,
			})
	}
	tracerEnabled := getTracerEnabled()
	if tracerEnabled != "" {
		envVals = append(envVals,
			corev1.EnvVar{
				Name:  tracerEnabledEnv,
				Value: tracerEnabled,
			})
	}

	jaegerPort := getJaegerPort()
	if jaegerPort != "" {
		envVals = append(envVals,
			corev1.EnvVar{
				Name:  jaegerPortEnv,
				Value: jaegerPort,
			})
	}

	value := []corev1.Container{{
		Name:            initContainerName,
		Image:           fmt.Sprintf("%s/%s:%s", getRepo(), getInitContainer(), getTag()),
		ImagePullPolicy: corev1.PullIfNotPresent,
		VolumeMounts: []corev1.VolumeMount{{
			ReadOnly:  true,
			Name:      "nsm-podinfo-volume",
			MountPath: "/etc/podinfo",
		}},
		Env:             envVals,
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"networkservicemesh.io/socket": resource.MustParse("1"),
			},
		},
	}}

	patch = append(patch, patchOperation{
		Op:    "add",
		Path:  initContainersPath,
		Value: value,
	})

	return patch
}

func addVolume(spec *corev1.PodSpec, added []corev1.Volume) (patch []patchOperation) {
	first := len(spec.Volumes) == 0
	var value interface{}
	for i := 0; i < len(added); i++ {
		value = added[i]
		path := volumePath
		if first {
			first = false
			value = []corev1.Volume{added[i]}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func addContainer(spec *corev1.PodSpec, containers []corev1.Container) (patch []patchOperation) {
	first := len(spec.Containers) == 0
	for i := 0; i < len(containers); i++ {
		value := &containers[i]
		path := containersPath
		if first {
			first = false
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}

	return patch
}
