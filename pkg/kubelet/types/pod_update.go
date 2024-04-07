/*
Copyright 2014 The Kubernetes Authors.

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

package types

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/scheduling"
)

// Annotation keys for annotations used in this package.
// note：Kubernetes中Pod配置相关的注解键。注解（Annotations）在Kubernetes资源中用于存储额外的非标识性元数据，这些元数据可以由工具、库、或者用户自定义。
const (
	ConfigSourceAnnotationKey    = "kubernetes.io/config.source"
	ConfigMirrorAnnotationKey    = v1.MirrorPodAnnotationKey
	ConfigFirstSeenAnnotationKey = "kubernetes.io/config.seen"
	ConfigHashAnnotationKey      = "kubernetes.io/config.hash"
)

// PodOperation defines what changes will be made on a pod configuration.
type PodOperation int

// These constants identify the PodOperations that can be made on a pod configuration.
// note：pod的操作，从0开始
const (
	// SET is the current pod configuration.
	SET PodOperation = iota
	// ADD signifies pods that are new to this source.
	ADD
	// DELETE signifies pods that are gracefully deleted from this source.
	DELETE
	// REMOVE signifies pods that have been removed from this source.
	REMOVE
	// UPDATE signifies pods have been updated in this source.
	UPDATE
	// RECONCILE signifies pods that have unexpected status in this source,
	// kubelet should reconcile status with this source.
	RECONCILE
)

// These constants identify the sources of pods.
// note：pods资源来源的方式，有三种：文件、http（网络上的）和api server
const (
	// Filesource idenitified updates from a file.
	FileSource = "file"
	// HTTPSource identifies updates from querying a web page.
	HTTPSource = "http"
	// ApiserverSource identifies updates from Kubernetes API Server.
	ApiserverSource = "api"
	// AllSource identifies updates from all sources.
	AllSource = "*"
)

// NamespaceDefault is a string representing the default namespace.
const NamespaceDefault = metav1.NamespaceDefault

// PodUpdate defines an operation sent on the channel. You can add or remove single services by
// sending an array of size one and Op == ADD|REMOVE (with REMOVE, only the ID is required).
// For setting the state of the system to a given state for this source configuration, set
// Pods as desired and Op to SET, which will reset the system state to that specified in this
// operation for this source channel. To remove all pods, set Pods to empty object and Op to SET.
//
// Additionally, Pods should never be nil - it should always point to an empty slice. While
// functionally similar, this helps our unit tests properly check that the correct PodUpdates
// are generated.
// note：
// 用于在一个通道（channel）上发送操作。这个结构体的设计允许通过发送一个包含单个服务的数组并指定操作类型（Op），
// 来添加或删除单个服务。具体的操作类型包括ADD和REMOVE，对于REMOVE操作，只需要指定服务的ID。
// 此外，还可以通过设置Pods为期望的状态并将Op设置为SET，来将系统的状态重置为此源配置指定的状态。如果要移除所有Pods，可以将Pods设置为一个空对象并将Op设置为SET。
type PodUpdate struct {
	Pods   []*v1.Pod
	Op     PodOperation
	Source string
}

// GetValidatedSources gets all validated sources from the specified sources.
func GetValidatedSources(sources []string) ([]string, error) {
	validated := make([]string, 0, len(sources))
	for _, source := range sources {
		switch source {
		case AllSource:
			return []string{FileSource, HTTPSource, ApiserverSource}, nil
		case FileSource, HTTPSource, ApiserverSource:
			validated = append(validated, source)
		case "":
			// Skip
		default:
			return []string{}, fmt.Errorf("unknown pod source %q", source)
		}
	}
	return validated, nil
}

// GetPodSource returns the source of the pod based on the annotation.
func GetPodSource(pod *v1.Pod) (string, error) {
	if pod.Annotations != nil {
		if source, ok := pod.Annotations[ConfigSourceAnnotationKey]; ok {
			return source, nil
		}
	}
	return "", fmt.Errorf("cannot get source of pod %q", pod.UID)
}

// SyncPodType classifies pod updates, eg: create, update.
// note：同步pod的操作
type SyncPodType int

const (
	// SyncPodSync is when the pod is synced to ensure desired state
	SyncPodSync SyncPodType = iota
	// SyncPodUpdate is when the pod is updated from source
	SyncPodUpdate
	// SyncPodCreate is when the pod is created from source
	SyncPodCreate
	// SyncPodKill is when the pod should have no running containers. A pod stopped in this way could be
	// restarted in the future due config changes.
	SyncPodKill
)

func (sp SyncPodType) String() string {
	switch sp {
	case SyncPodCreate:
		return "create"
	case SyncPodUpdate:
		return "update"
	case SyncPodSync:
		return "sync"
	case SyncPodKill:
		return "kill"
	default:
		return "unknown"
	}
}

// IsMirrorPod returns true if the passed Pod is a Mirror Pod.
// note：判断是否是镜像pod通过pod结构体的注解
func IsMirrorPod(pod *v1.Pod) bool {
	if pod.Annotations == nil {
		return false
	}
	_, ok := pod.Annotations[ConfigMirrorAnnotationKey]
	return ok
}

// IsStaticPod returns true if the pod is a static pod.
// note：通过判断是否是api创建来判断是否是静态pod
func IsStaticPod(pod *v1.Pod) bool {
	source, err := GetPodSource(pod)
	return err == nil && source != ApiserverSource
}

// IsCriticalPod returns true if pod's priority is greater than or equal to SystemCriticalPriority.
func IsCriticalPod(pod *v1.Pod) bool {
	if IsStaticPod(pod) {
		return true
	}
	if IsMirrorPod(pod) {
		return true
	}
	if pod.Spec.Priority != nil && IsCriticalPodBasedOnPriority(*pod.Spec.Priority) {
		return true
	}
	return false
}

// Preemptable returns true if preemptor pod can preempt preemptee pod
// if preemptee is not critical or if preemptor's priority is greater than preemptee's priority
func Preemptable(preemptor, preemptee *v1.Pod) bool {
	if IsCriticalPod(preemptor) && !IsCriticalPod(preemptee) {
		return true
	}
	if (preemptor != nil && preemptor.Spec.Priority != nil) &&
		(preemptee != nil && preemptee.Spec.Priority != nil) {
		return *(preemptor.Spec.Priority) > *(preemptee.Spec.Priority)
	}

	return false
}

// IsCriticalPodBasedOnPriority checks if the given pod is a critical pod based on priority resolved from pod Spec.
func IsCriticalPodBasedOnPriority(priority int32) bool {
	return priority >= scheduling.SystemCriticalPriority
}

// IsNodeCriticalPod checks if the given pod is a system-node-critical
func IsNodeCriticalPod(pod *v1.Pod) bool {
	return IsCriticalPod(pod) && (pod.Spec.PriorityClassName == scheduling.SystemNodeCritical)
}

// IsRestartableInitContainer returns true if the initContainer has
// ContainerRestartPolicyAlways.
func IsRestartableInitContainer(initContainer *v1.Container) bool {
	if initContainer.RestartPolicy == nil {
		return false
	}

	return *initContainer.RestartPolicy == v1.ContainerRestartPolicyAlways
}
