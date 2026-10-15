// Copyright 2026 Nieuwsma Services LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCollapseFamily(t *testing.T) {
	tests := map[string]string{
		"rize-rack-003-slot-01": "rize-rack-003-slot-*",
		"ibm-rack-003-slot-32":  "ibm-rack-003-slot-*",
		"rack-realizer":         "rack-realizer",
	}
	for input, want := range tests {
		if got := collapseFamily(input); got != want {
			t.Errorf("collapseFamily(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestHierarchyLevelCycles(t *testing.T) {
	level := levelTotal
	for _, want := range []hierarchyLevel{levelClusters, levelNamespaces, levelApps, levelTotal} {
		level = level.next()
		if level != want {
			t.Fatalf("next level = %s, want %s", level, want)
		}
	}
	if got := levelTotal.previous(); got != levelApps {
		t.Fatalf("previous from total = %s, want apps", got)
	}
}

func TestTotalUsesSharedHostCapacity(t *testing.T) {
	clusters := []*clusterUsage{
		{NodeCPU: 1_000_000_000, CPUCapacity: 10_000_000_000, NodeMemory: 2 << 30, MemoryCapacity: 20 << 30, Pods: 2, ReadyPods: 2},
		{NodeCPU: 2_000_000_000, CPUCapacity: 10_000_000_000, NodeMemory: 3 << 30, MemoryCapacity: 20 << 30, Pods: 3, ReadyPods: 2},
		{Error: errForTest{}},
	}
	got := totalUsage(clusters)
	if got.cpu != 3_000_000_000 || got.cpuCapacity != 10_000_000_000 {
		t.Fatalf("CPU total/capacity = %d/%d", got.cpu, got.cpuCapacity)
	}
	if got.memory != 5<<30 || got.memoryCapacity != 20<<30 {
		t.Fatalf("memory total/capacity = %d/%d", got.memory, got.memoryCapacity)
	}
	if got.clusters != 3 || got.readyClusters != 2 || got.pods != 5 || got.readyPods != 4 {
		t.Fatalf("unexpected totals: %+v", got)
	}
}

type errForTest struct{}

func (errForTest) Error() string { return "unavailable" }

func TestQuantities(t *testing.T) {
	if got := parseCPU("10"); got != 10_000_000_000 {
		t.Fatalf("parseCPU(10) = %d", got)
	}
	if got := parseCPU("250m"); got != 250_000_000 {
		t.Fatalf("parseCPU(250m) = %d", got)
	}
	if got := parseBytes("2Gi"); got != 2<<30 {
		t.Fatalf("parseBytes(2Gi) = %d", got)
	}
	if got := parseBytes("512Mi"); got != 512<<20 {
		t.Fatalf("parseBytes(512Mi) = %d", got)
	}
}

func TestDisplayRowsShareMetricColumns(t *testing.T) {
	const nameWidth = 44
	rows := []displayRow{
		{name: "● cluster", cpu: "671m / 10.00", memory: "5.92G / 19.49G"},
		{name: "    application-with-a-longer-name", cpu: "2m", memory: "20M"},
	}
	for _, row := range rows {
		var output bytes.Buffer
		writeDisplayRow(&output, nameWidth, row, "")
		line := strings.TrimSuffix(output.String(), "\n")
		cpuByteEnd := strings.Index(line, row.cpu) + len(row.cpu)
		memoryByteEnd := strings.Index(line, row.memory) + len(row.memory)
		cpuEnd := utf8.RuneCountInString(line[:cpuByteEnd])
		memoryEnd := utf8.RuneCountInString(line[:memoryByteEnd])
		if want := nameWidth + 2 + cpuColumnWidth; cpuEnd != want {
			t.Errorf("CPU column for %q ends at %d, want %d", row.name, cpuEnd, want)
		}
		if want := nameWidth + 2 + cpuColumnWidth + 2 + memoryColumnWidth; memoryEnd != want {
			t.Errorf("memory column for %q ends at %d, want %d", row.name, memoryEnd, want)
		}
	}
}

func TestAggregateGroupsControllerFamiliesAndHaltedWorkloads(t *testing.T) {
	objects := []k8sObject{
		controller("Deployment", "rack-simulator", "rize-rack-003-slot-01", 1),
		controller("Deployment", "rack-simulator", "rize-rack-003-slot-02", 1),
		controller("Deployment", "arc-reference", "reference", 0),
		replicaSet("rack-simulator", "rize-rack-003-slot-01-aaaa", "rize-rack-003-slot-01"),
		replicaSet("rack-simulator", "rize-rack-003-slot-02-bbbb", "rize-rack-003-slot-02"),
		pod("rack-simulator", "rize-01-pod", "uid-1", "rize-rack-003-slot-01-aaaa", 4, true),
		pod("rack-simulator", "rize-02-pod", "uid-2", "rize-rack-003-slot-02-bbbb", 4, false),
		pod("kube-system", "coredns", "uid-system", "", 1, true),
	}
	var summary nodeSummary
	summary.Node.CPU = &struct {
		UsageNanoCores uint64 `json:"usageNanoCores"`
	}{UsageNanoCores: 900_000_000}
	summary.Node.Memory = &struct {
		WorkingSetBytes uint64 `json:"workingSetBytes"`
	}{WorkingSetBytes: 2 << 30}
	summary.Pods = []summaryPod{
		summaryMetric("rack-simulator", "rize-01-pod", "uid-1", 100_000_000, 100<<20),
		summaryMetric("rack-simulator", "rize-02-pod", "uid-2", 200_000_000, 200<<20),
		summaryMetric("kube-system", "coredns", "uid-system", 50_000_000, 50<<20),
	}

	got := aggregate(&clusterUsage{Name: "whoiz"}, objects, []nodeSummary{summary}, false)
	if len(got.Namespaces) != 2 {
		t.Fatalf("namespaces = %d, want 2", len(got.Namespaces))
	}

	rack := findNamespace(t, got, "rack-simulator")
	if len(rack.Apps) != 1 {
		t.Fatalf("rack apps = %d, want 1", len(rack.Apps))
	}
	app := rack.Apps[0]
	if app.Name != "rize-rack-003-slot-*" || app.Workloads != 2 || app.Pods != 2 || app.ReadyPods != 1 || app.Containers != 8 {
		t.Fatalf("unexpected grouped app: %+v", app)
	}
	if app.CPU != 300_000_000 || app.Memory != 300<<20 {
		t.Fatalf("unexpected grouped usage: %+v", app)
	}

	arc := findNamespace(t, got, "arc-reference")
	if len(arc.Apps) != 1 || arc.Apps[0].Name != "reference" || arc.Apps[0].Pods != 0 || arc.Apps[0].Desired != 0 {
		t.Fatalf("halted application not retained: %+v", arc.Apps)
	}
}

func TestAggregateSumsRequestsAndLimits(t *testing.T) {
	object := pod("rack-simulator", "rize-01-pod", "uid-1", "rize-rack-003-slot-01-aaaa", 1, true)
	object.Spec.Containers[0].Resources.Requests = map[string]string{"cpu": "100m", "memory": "64Mi"}
	object.Spec.Containers[0].Resources.Limits = map[string]string{"memory": "128Mi"}
	objects := []k8sObject{
		controller("Deployment", "rack-simulator", "rize-rack-003-slot-01", 1),
		replicaSet("rack-simulator", "rize-rack-003-slot-01-aaaa", "rize-rack-003-slot-01"),
		object,
	}
	got := aggregate(&clusterUsage{Name: "whoiz"}, objects, nil, false)
	app := findNamespace(t, got, "rack-simulator").Apps[0]
	if app.CPURequest != 100_000_000 || app.MemoryRequest != 64<<20 || app.MemoryLimit != 128<<20 {
		t.Fatalf("requests and limits = cpu %d mem %d limit %d", app.CPURequest, app.MemoryRequest, app.MemoryLimit)
	}
	if got.MemoryRequest != app.MemoryRequest || got.MemoryLimit != app.MemoryLimit {
		t.Fatalf("cluster rollup = %+v", got)
	}
}

func TestFilterAndSort(t *testing.T) {
	clusters := []*clusterUsage{{
		Name: "whoiz", Context: "kind-whoiz",
		Namespaces: []*namespaceUsage{
			{Name: "quiet", Memory: 10, Apps: []*appUsage{{Name: "reader", Memory: 10, CPU: 9}}},
			{Name: "rack-simulator", Memory: 5, CPU: 80, Apps: []*appUsage{
				{Name: "slot-keeper", Memory: 5, CPU: 1},
				{Name: "rize-rack-003-slot-*", Memory: 1, CPU: 90},
			}},
		},
	}}
	filtered := filterClusters(clusters, "rize")
	if len(filtered) != 1 || len(filtered[0].Namespaces) != 1 || len(filtered[0].Namespaces[0].Apps) != 1 {
		t.Fatalf("filter = %+v", filtered)
	}
	if filtered[0].Namespaces[0].Apps[0].Name != "rize-rack-003-slot-*" {
		t.Fatalf("filtered app = %s", filtered[0].Namespaces[0].Apps[0].Name)
	}
	sortClusters(clusters, "cpu")
	var rack *namespaceUsage
	for _, namespace := range clusters[0].Namespaces {
		if namespace.Name == "rack-simulator" {
			rack = namespace
		}
	}
	if rack == nil || rack.Apps[0].Name != "rize-rack-003-slot-*" {
		t.Fatalf("cpu sort = %+v", rack)
	}
	sortClusters(clusters, "name")
	if clusters[0].Namespaces[0].Name != "quiet" {
		t.Fatalf("name sort namespace = %s", clusters[0].Namespaces[0].Name)
	}
}

func TestMemoryDeltaUsesThePreviousSample(t *testing.T) {
	previous := map[string]*clusterUsage{
		"kind-whoiz": {NodeMemory: 200, Namespaces: []*namespaceUsage{{
			Name: "rack", Memory: 80, Apps: []*appUsage{{Name: "slot-*", Memory: 80}},
		}}},
	}
	current := []*clusterUsage{{
		Context: "kind-whoiz", NodeMemory: 250, Namespaces: []*namespaceUsage{{
			Name: "rack", Memory: 50, Apps: []*appUsage{{Name: "slot-*", Memory: 50}},
		}},
	}}
	applyDeltas(current, previous)
	if current[0].MemoryDelta != 50 || current[0].Namespaces[0].MemoryDelta != -30 || current[0].Namespaces[0].Apps[0].MemoryDelta != -30 {
		t.Fatalf("deltas = cluster %d ns %d app %d", current[0].MemoryDelta, current[0].Namespaces[0].MemoryDelta, current[0].Namespaces[0].Apps[0].MemoryDelta)
	}
}

func TestJSONSnapshot(t *testing.T) {
	var output bytes.Buffer
	clusters := []*clusterUsage{{
		Name: "whoiz", Context: "kind-whoiz", MemoryRequest: 64, MemoryDelta: -10,
		Namespaces: []*namespaceUsage{{Name: "rack", Apps: []*appUsage{{Name: "slot-*", MemoryRequest: 64}}}},
	}}
	if err := writeJSON(&output, clusters, options{sortKey: "memory", filter: "slot"}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{`"sort": "memory"`, `"filter": "slot"`, `"memory_delta_bytes": -10`, `"name": "slot-*"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("json missing %s\n%s", want, text)
		}
	}
}

func controller(kind, namespace, name string, replicas int64) k8sObject {
	object := k8sObject{Kind: kind, Metadata: objectMeta{Namespace: namespace, Name: name}}
	object.Spec.Replicas = &replicas
	return object
}

func replicaSet(namespace, name, deployment string) k8sObject {
	return k8sObject{
		Kind: "ReplicaSet",
		Metadata: objectMeta{
			Namespace: namespace,
			Name:      name,
			OwnerReferences: []ownerReference{{
				Kind: "Deployment", Name: deployment, Controller: true,
			}},
		},
	}
}

func pod(namespace, name, uid, replicaSetName string, containers int, ready bool) k8sObject {
	object := k8sObject{Kind: "Pod", Metadata: objectMeta{Namespace: namespace, Name: name, UID: uid}}
	if replicaSetName != "" {
		object.Metadata.OwnerReferences = []ownerReference{{Kind: "ReplicaSet", Name: replicaSetName, Controller: true}}
	}
	object.Spec.Containers = make([]containerSpec, containers)
	status := "False"
	if ready {
		status = "True"
	}
	object.Status.Conditions = []condition{{Type: "Ready", Status: status}}
	return object
}

func summaryMetric(namespace, name, uid string, cpu, memory uint64) summaryPod {
	var pod summaryPod
	pod.PodRef.Namespace = namespace
	pod.PodRef.Name = name
	pod.PodRef.UID = uid
	pod.CPU = &struct {
		UsageNanoCores uint64 `json:"usageNanoCores"`
	}{UsageNanoCores: cpu}
	pod.Memory = &struct {
		WorkingSetBytes uint64 `json:"workingSetBytes"`
	}{WorkingSetBytes: memory}
	return pod
}

func findNamespace(t *testing.T, cluster *clusterUsage, name string) *namespaceUsage {
	t.Helper()
	for _, namespace := range cluster.Namespaces {
		if namespace.Name == name {
			return namespace
		}
	}
	t.Fatalf("namespace %q not found", name)
	return nil
}
