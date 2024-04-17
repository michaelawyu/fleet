package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	placementv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
)

const (
	crpCount    = 120
	workerCount = 20

	namespaceNameTmpl = "work-%d"
	configMapNameTmpl = "data-%d"
	crpNameTmpl       = "crp-%d"

	envLabelName = "environment"
	locLabelName = "location"

	memberClusterCount = 25
)

var (
	perCRPProcessingTimeLimit = time.Minute * 5

	envLabelValues = []string{"dev", "staging", "prod"}
	locLabelValues = []string{"eastus", "westus", "centralus", "northeurope", "eastasia"}
)

var (
	hubClient client.Client
	tracker   *latencyTracker
)

type latencyTracker struct {
	mu sync.Mutex

	succeededCRPCount int
	latencies         []time.Duration
}

func (t *latencyTracker) record(latency time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.succeededCRPCount++
	t.latencies = append(t.latencies, latency)
}

func (t *latencyTracker) report() (succeededCRPCount int, quantile50, quantile75, quantile90, quantile95, quantile99 time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.succeededCRPCount == 0 {
		return 0, 0, 0, 0, 0, 0
	}

	sort.Slice(t.latencies, func(i, j int) bool {
		return t.latencies[i] < t.latencies[j]
	})

	quantile50 = t.latencies[int(math.Floor(float64(t.succeededCRPCount)*0.5))]
	quantile75 = t.latencies[int(math.Floor(float64(t.succeededCRPCount)*0.75))]
	quantile90 = t.latencies[int(math.Floor(float64(t.succeededCRPCount)*0.9))]
	quantile95 = t.latencies[int(math.Floor(float64(t.succeededCRPCount)*0.95))]
	quantile99 = t.latencies[int(math.Floor(float64(t.succeededCRPCount)*0.99))]
	return t.succeededCRPCount, quantile50, quantile75, quantile90, quantile95, quantile99
}

func init() {
	klog.InitFlags(nil)

	utilrand.Seed(time.Now().UnixNano())

	utilruntime.Must(placementv1beta1.AddToScheme(scheme.Scheme))
}

func main() {
	flag.Parse()
	defer klog.Flush()

	klog.Info("Preparing K8s client")
	config := config.GetConfigOrDie()
	config.QPS, config.Burst = float32(100), 500 //queries per second, max # of queries queued at once
	var err error
	hubClient, err = client.New(config, client.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create K8s client: %v", err))
	}

	tracker = &latencyTracker{
		succeededCRPCount: 0,
		latencies:         make([]time.Duration, 0, crpCount),
	}

	ctx := ctrl.SetupSignalHandler()
	terminated := ctx.Done()
	var wg sync.WaitGroup

	klog.Info("Cleaning things up")
	cleanup(ctx)

	klog.Info("Pause for 20 seconds")
	time.Sleep(time.Second * 20)

	klog.Info("Starting the workers")
	for i := 0; i < workerCount; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()

			j := i
			for ; j < crpCount; j += workerCount {
				childCtx, childCancelFunc := context.WithDeadline(ctx, time.Now().Add(perCRPProcessingTimeLimit))
				timedOut := childCtx.Done()
				defer childCancelFunc()
				select {
				case <-terminated:
					klog.Infof("Worker %d is terminated", i)
					return
				case <-timedOut:
					klog.Infof("Worker %d has timed out when processing tasks %d", i, j)
					return
				default:
					klog.Infof("Worker %d has started processing task %d", i, j)
					if err := work(childCtx, j); err != nil {
						klog.Errorf("Worker %d failed to process task %d: %v", i, j, err)
					}
				}
			}
			klog.Infof("Worker %d has completed", i)
		}()
	}

	wg.Wait()
	//klog.Info("Cleaning things up")
	//cleanup(ctx)

	succeededCRPCount, quantile50, quantile75, quantile90, quantile95, quantile99 := tracker.report()
	klog.Infof("Summary: %d CRPs have been successfully applied", succeededCRPCount)
	klog.Infof("Latency: 50th percentile: %v", quantile50)
	klog.Infof("Latency: 75th percentile: %v", quantile75)
	klog.Infof("Latency: 90th percentile: %v", quantile90)
	klog.Infof("Latency: 95th percentile: %v", quantile95)
	klog.Infof("Latency: 99th percentile: %v", quantile99)
}

func work(ctx context.Context, idx int) error {
	klog.Infof("Processing task %d", idx)

	klog.Infof("Creating the resources for task %d", idx)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf(namespaceNameTmpl, idx),
		},
	}
	if err := hubClient.Create(ctx, ns); err != nil {
		return fmt.Errorf("failed to create namespace: %v", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(configMapNameTmpl, idx),
			Namespace: fmt.Sprintf(namespaceNameTmpl, idx),
		},
		Data: map[string]string{
			"key": "value",
		},
	}
	if err := hubClient.Create(ctx, cm); err != nil {
		return fmt.Errorf("failed to create configmap: %v", err)
	}

	klog.Infof("Creating the CRP for task %d", idx)
	crp := &placementv1beta1.ClusterResourcePlacement{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf(crpNameTmpl, idx),
		},
		Spec: placementv1beta1.ClusterResourcePlacementSpec{
			ResourceSelectors: []placementv1beta1.ClusterResourceSelector{
				{
					Group:   "",
					Version: "v1",
					Kind:    "Namespace",
					Name:    fmt.Sprintf(namespaceNameTmpl, idx),
				},
			},
			Policy: &placementv1beta1.PlacementPolicy{
				PlacementType:    placementv1beta1.PickNPlacementType,
				NumberOfClusters: ptr.To(int32(utilrand.IntnRange(8, 15))),
				Affinity: &placementv1beta1.Affinity{
					ClusterAffinity: &placementv1beta1.ClusterAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []placementv1beta1.PreferredClusterSelector{
							{
								Weight: 100,
								Preference: placementv1beta1.ClusterSelectorTerm{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											envLabelName: envLabelValues[idx%len(envLabelValues)],
										},
									},
								},
							},
							{
								Weight: 100,
								Preference: placementv1beta1.ClusterSelectorTerm{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											locLabelName: locLabelValues[idx%len(locLabelValues)],
										},
									},
								},
							},
						},
					},
				},
			},
			Strategy: placementv1beta1.RolloutStrategy{
				Type: placementv1beta1.RollingUpdateRolloutStrategyType,
				RollingUpdate: &placementv1beta1.RollingUpdateConfig{
					MaxUnavailable:           ptr.To(intstr.FromInt(memberClusterCount)),
					MaxSurge:                 ptr.To(intstr.FromInt(memberClusterCount)),
					UnavailablePeriodSeconds: ptr.To(1),
				},
			},
		},
	}
	startTime := time.Now()
	if err := hubClient.Create(ctx, crp); err != nil {
		return fmt.Errorf("failed to create CRP: %v", err)
	}

	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			klog.Infof("Task %d is terminated", idx)
			return nil
		case <-ticker.C:
			polledCRP := &placementv1beta1.ClusterResourcePlacement{}
			if err := hubClient.Get(ctx, client.ObjectKey{Name: crp.Name}, polledCRP); err != nil {
				klog.ErrorS(err, "Failed to poll CRP", "CRP", crp.Name)
				continue
			}

			appliedCond := polledCRP.GetCondition(string(placementv1beta1.ClusterResourcePlacementAppliedConditionType))
			//klog.Info(fmt.Sprintf("%+v", appliedCond))
			if appliedCond != nil && appliedCond.Status == metav1.ConditionTrue {
				klog.Infof("CRP %s has been applied", crp.Name)
				tracker.record(time.Since(startTime))
				return nil
			}
			klog.Infof("CRP %s is not yet applied", crp.Name)
		}
	}
}

func cleanup(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := i; j < crpCount; j += workerCount {
				klog.Infof("Cleaning up task %d", i)
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf(namespaceNameTmpl, j),
					},
				}
				if err := hubClient.Delete(ctx, ns); client.IgnoreNotFound(err) != nil {
					klog.Errorf("Failed to delete namespace %s: %v", ns.Name, err)
				}

				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf(configMapNameTmpl, j),
						Namespace: fmt.Sprintf(namespaceNameTmpl, j),
					},
				}
				if err := hubClient.Delete(ctx, cm); client.IgnoreNotFound(err) != nil {
					klog.Errorf("Failed to delete configmap %s: %v", cm.Name, err)
				}

				crp := &placementv1beta1.ClusterResourcePlacement{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf(crpNameTmpl, j),
					},
				}
				if err := hubClient.Delete(ctx, crp); client.IgnoreNotFound(err) != nil {
					klog.Errorf("Failed to delete CRP %s: %v", crp.Name, err)
				}

				ticker := time.NewTicker(time.Second * 10)
				defer ticker.Stop()
				for {
					shouldEscape := false

					select {
					case <-ctx.Done():
						klog.Infof("Cleanup for task %d is terminated", j)
						return
					case <-ticker.C:
						polledNS := &corev1.Namespace{}
						if err := hubClient.Get(ctx, client.ObjectKeyFromObject(ns), polledNS); !errors.IsNotFound(err) {
							klog.Infof("Namespace for task %d still exists", j)
							continue
						}

						polledCM := &corev1.ConfigMap{}
						if err := hubClient.Get(ctx, client.ObjectKeyFromObject(cm), polledCM); !errors.IsNotFound(err) {
							klog.Infof("ConfigMap for task %d still exists", j)
							continue
						}

						polledCRP := &placementv1beta1.ClusterResourcePlacement{}
						if err := hubClient.Get(ctx, client.ObjectKeyFromObject(crp), polledCRP); !errors.IsNotFound(err) {
							klog.Infof("CRP for task %d still exists", j)
							continue
						}

						klog.Infof("All resources related to task %d have been cleaned up", j)
						shouldEscape = true
					}

					if shouldEscape {
						break
					}
				}
			}
		}()
	}
	wg.Wait()
}
