// Copyright 2026 Nieuwsma Services LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const topologyResources = "pods,replicasets,deployments,statefulsets,daemonsets,jobs,nodes"

var slotFamilyPattern = regexp.MustCompile(`^(.*-slot-)\d+($|-.+)$`)
var version = "0.0.0-local"
var revision = "unknown"

type options struct {
	kubectl         string
	contexts        string
	interval        time.Duration
	topologyRefresh time.Duration
	timeout         time.Duration
	level           hierarchyLevel
	sortKey         string
	filter          string
	once            bool
	asJSON          bool
	allNamespaces   bool
	noColor         bool
	showVersion     bool
}

type hierarchyLevel int

const (
	levelTotal hierarchyLevel = iota
	levelClusters
	levelNamespaces
	levelApps
)

func (level hierarchyLevel) String() string {
	switch level {
	case levelTotal:
		return "TOTAL"
	case levelClusters:
		return "CLUSTERS"
	case levelNamespaces:
		return "NAMESPACES"
	default:
		return "APPS"
	}
}

func (level hierarchyLevel) next() hierarchyLevel {
	return (level + 1) % 4
}

func (level hierarchyLevel) previous() hierarchyLevel {
	return (level + 3) % 4
}

type levelFlag struct {
	value *hierarchyLevel
}

func (flagValue levelFlag) String() string {
	if flagValue.value == nil {
		return levelApps.String()
	}
	return flagValue.value.String()
}

func (flagValue levelFlag) Set(value string) error {
	switch strings.ToLower(value) {
	case "total":
		*flagValue.value = levelTotal
	case "clusters", "cluster":
		*flagValue.value = levelClusters
	case "namespaces", "namespace":
		*flagValue.value = levelNamespaces
	case "apps", "app", "applications":
		*flagValue.value = levelApps
	default:
		return fmt.Errorf("invalid level %q: use total, clusters, namespaces, or apps", value)
	}
	return nil
}

type ownerReference struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Controller bool   `json:"controller"`
}

type objectMeta struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	UID             string            `json:"uid"`
	Labels          map[string]string `json:"labels"`
	OwnerReferences []ownerReference  `json:"ownerReferences"`
}

type containerSpec struct {
	Name      string `json:"name"`
	Resources struct {
		Requests map[string]string `json:"requests"`
		Limits   map[string]string `json:"limits"`
	} `json:"resources"`
}

type condition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type k8sObject struct {
	Kind     string     `json:"kind"`
	Metadata objectMeta `json:"metadata"`
	Spec     struct {
		NodeName   string          `json:"nodeName"`
		Replicas   *int64          `json:"replicas"`
		Containers []containerSpec `json:"containers"`
		Template   struct {
			Spec struct {
				Containers []containerSpec `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		Phase      string            `json:"phase"`
		Conditions []condition       `json:"conditions"`
		Capacity   map[string]string `json:"capacity"`
		Active     int64             `json:"active"`
	} `json:"status"`
}

type objectList struct {
	Items []k8sObject `json:"items"`
}

type usage struct {
	CPU *struct {
		UsageNanoCores uint64 `json:"usageNanoCores"`
	} `json:"cpu"`
	Memory *struct {
		WorkingSetBytes uint64 `json:"workingSetBytes"`
	} `json:"memory"`
}

type summaryPod struct {
	PodRef struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		UID       string `json:"uid"`
	} `json:"podRef"`
	CPU *struct {
		UsageNanoCores uint64 `json:"usageNanoCores"`
	} `json:"cpu"`
	Memory *struct {
		WorkingSetBytes uint64 `json:"workingSetBytes"`
	} `json:"memory"`
}

type nodeSummary struct {
	Node struct {
		NodeName string `json:"nodeName"`
		CPU      *struct {
			UsageNanoCores uint64 `json:"usageNanoCores"`
		} `json:"cpu"`
		Memory *struct {
			WorkingSetBytes uint64 `json:"workingSetBytes"`
		} `json:"memory"`
	} `json:"node"`
	Pods []summaryPod `json:"pods"`
}

type topology struct {
	objects   []k8sObject
	fetchedAt time.Time
}

type collector struct {
	options  options
	mu       sync.Mutex
	cache    map[string]topology
	previous map[string]*clusterUsage
	latest   []*clusterUsage
}

type workloadKey struct {
	Namespace string
	Kind      string
	Name      string
}

type appUsage struct {
	Name       string
	Kinds      map[string]bool
	CPU        uint64
	Memory     uint64
	Pods       int
	ReadyPods  int
	Containers int
	Workloads     int
	Desired       int64
	CPURequest    uint64
	MemoryRequest uint64
	CPULimit      uint64
	MemoryLimit   uint64
	MemoryDelta   int64
}

type namespaceUsage struct {
	Name       string
	CPU        uint64
	Memory     uint64
	Pods       int
	ReadyPods  int
	Containers    int
	CPURequest    uint64
	MemoryRequest uint64
	CPULimit      uint64
	MemoryLimit   uint64
	MemoryDelta   int64
	Apps          []*appUsage
}

type clusterUsage struct {
	Context        string
	Name           string
	Error          error
	NodeCPU        uint64
	NodeMemory     uint64
	CPUCapacity    uint64
	MemoryCapacity uint64
	AppCPU         uint64
	AppMemory      uint64
	Pods           int
	ReadyPods      int
	Containers     int
	CPURequest     uint64
	MemoryRequest  uint64
	CPULimit       uint64
	MemoryLimit    uint64
	MemoryDelta    int64
	Namespaces     []*namespaceUsage
}

func main() {
	opts := parseFlags()
	if opts.showVersion {
		fmt.Printf("kind-app-top %s (%s)\n", version, revision)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	contexts, err := discoverContexts(ctx, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kat:", err)
		os.Exit(1)
	}
	if len(contexts) == 0 {
		fmt.Fprintln(os.Stderr, "kat: no kind-* kubeconfig contexts found")
		os.Exit(1)
	}

	c := &collector{options: opts, cache: make(map[string]topology), previous: map[string]*clusterUsage{}}
	started := time.Now()
	elapsed := time.Since(started)
	clusters := c.refresh(ctx, contexts)
	elapsed = time.Since(started)
	if opts.asJSON {
		if err := writeJSON(os.Stdout, clusters, opts); err != nil {
			fmt.Fprintln(os.Stderr, "kat:", err)
			os.Exit(1)
		}
		return
	}
	render(os.Stdout, clusters, opts, elapsed)
	if opts.once {
		return
	}

	keys, restoreTerminal := keyboardInput()
	defer restoreTerminal()
	ticker := time.NewTicker(opts.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case key, ok := <-keys:
			if !ok {
				keys = nil
				continue
			}
			switch key {
			case ']':
				opts.level = opts.level.next()
			case '[':
				opts.level = opts.level.previous()
			case 'm', 'M':
				opts.sortKey = "memory"
			case 'c', 'C':
				opts.sortKey = "cpu"
			case 'n', 'N':
				opts.sortKey = "name"
			case 'q', 'Q':
				return
			default:
				continue
			}
			c.options = opts
			render(os.Stdout, c.present(), opts, elapsed)
		case <-ticker.C:
			started = time.Now()
			clusters = c.refresh(ctx, contexts)
			elapsed = time.Since(started)
			render(os.Stdout, clusters, opts, elapsed)
		}
	}
}

func (c *collector) refresh(ctx context.Context, contexts []string) []*clusterUsage {
	raw := c.collectAll(ctx, contexts)
	applyDeltas(raw, c.previous)
	c.previous = indexClusters(raw)
	c.latest = raw
	c.options.sortKey = normalizeSort(c.options.sortKey)
	return c.present()
}

func (c *collector) present() []*clusterUsage {
	view := filterClusters(c.latest, c.options.filter)
	sortClusters(view, c.options.sortKey)
	return view
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.kubectl, "kubectl", "kubectl", "kubectl executable")
	flag.StringVar(&opts.contexts, "contexts", "", "comma-separated contexts (default: every kind-* context)")
	flag.DurationVar(&opts.interval, "interval", 5*time.Second, "refresh interval")
	flag.DurationVar(&opts.topologyRefresh, "topology-refresh", 30*time.Second, "workload topology refresh interval")
	flag.DurationVar(&opts.timeout, "timeout", 20*time.Second, "timeout per Kubernetes request")
	opts.level = levelApps
	flag.Var(levelFlag{value: &opts.level}, "level", "maximum hierarchy level: total, clusters, namespaces, or apps")
	flag.StringVar(&opts.sortKey, "sort", "memory", "sort rows by memory, cpu, or name")
	flag.StringVar(&opts.filter, "filter", "", "show namespaces or apps whose names contain this text")
	flag.BoolVar(&opts.once, "once", false, "render once and exit")
	flag.BoolVar(&opts.asJSON, "json", false, "print one JSON snapshot and exit")
	flag.BoolVar(&opts.allNamespaces, "all-namespaces", false, "include Kubernetes system namespaces")
	flag.BoolVar(&opts.noColor, "no-color", os.Getenv("NO_COLOR") != "", "disable ANSI color")
	flag.BoolVar(&opts.showVersion, "version", false, "print version and source revision")
	flag.Parse()
	opts.sortKey = normalizeSort(opts.sortKey)
	if opts.sortKey == "" {
		fmt.Fprintln(os.Stderr, "kat: --sort must be memory, cpu, or name")
		os.Exit(1)
	}
	return opts
}

func normalizeSort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "memory", "mem":
		return "memory"
	case "cpu":
		return "cpu"
	case "name":
		return "name"
	default:
		return ""
	}
}

func discoverContexts(ctx context.Context, opts options) ([]string, error) {
	if strings.TrimSpace(opts.contexts) != "" {
		var contexts []string
		for _, value := range strings.Split(opts.contexts, ",") {
			if value = strings.TrimSpace(value); value != "" {
				contexts = append(contexts, value)
			}
		}
		return contexts, nil
	}
	out, err := run(ctx, opts.timeout, opts.kubectl, "config", "get-contexts", "-o", "name")
	if err != nil {
		return nil, err
	}
	var contexts []string
	for _, value := range strings.Fields(string(out)) {
		if strings.HasPrefix(value, "kind-") {
			contexts = append(contexts, value)
		}
	}
	sort.Strings(contexts)
	return contexts, nil
}

func (c *collector) collectAll(ctx context.Context, contexts []string) []*clusterUsage {
	results := make([]*clusterUsage, len(contexts))
	var wg sync.WaitGroup
	for i, kubeContext := range contexts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = c.collectCluster(ctx, kubeContext)
		}()
	}
	wg.Wait()
	sort.SliceStable(results, func(i, j int) bool {
		if (results[i].Error == nil) != (results[j].Error == nil) {
			return results[i].Error == nil
		}
		if results[i].NodeMemory != results[j].NodeMemory {
			return results[i].NodeMemory > results[j].NodeMemory
		}
		return results[i].Name < results[j].Name
	})
	return results
}

func (c *collector) collectCluster(ctx context.Context, kubeContext string) *clusterUsage {
	result := &clusterUsage{Context: kubeContext, Name: strings.TrimPrefix(kubeContext, "kind-")}
	objects, err := c.getTopology(ctx, kubeContext)
	if err != nil {
		result.Error = err
		return result
	}

	var nodes []string
	for _, object := range objects {
		if object.Kind != "Node" {
			continue
		}
		nodes = append(nodes, object.Metadata.Name)
		result.CPUCapacity += parseCPU(object.Status.Capacity["cpu"])
		result.MemoryCapacity += parseBytes(object.Status.Capacity["memory"])
	}
	if len(nodes) == 0 {
		result.Error = errors.New("no nodes found")
		return result
	}

	summaries := make([]nodeSummary, len(nodes))
	var wg sync.WaitGroup
	errCh := make(chan error, len(nodes))
	for i, node := range nodes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path := "/api/v1/nodes/" + node + "/proxy/stats/summary"
			out, queryErr := run(ctx, c.options.timeout, c.options.kubectl, "--context", kubeContext, "get", "--raw", path)
			if queryErr != nil {
				errCh <- fmt.Errorf("node %s metrics: %w", node, queryErr)
				return
			}
			if queryErr = json.Unmarshal(out, &summaries[i]); queryErr != nil {
				errCh <- fmt.Errorf("node %s metrics JSON: %w", node, queryErr)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	if queryErr := <-errCh; queryErr != nil {
		result.Error = queryErr
		return result
	}

	return aggregate(result, objects, summaries, c.options.allNamespaces)
}

func (c *collector) getTopology(ctx context.Context, kubeContext string) ([]k8sObject, error) {
	c.mu.Lock()
	cached, ok := c.cache[kubeContext]
	c.mu.Unlock()
	if ok && time.Since(cached.fetchedAt) < c.options.topologyRefresh {
		return cached.objects, nil
	}
	out, err := run(ctx, c.options.timeout, c.options.kubectl,
		"--context", kubeContext, "get", topologyResources, "-A", "-o", "json")
	if err != nil {
		return nil, err
	}
	var list objectList
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("workload topology JSON: %w", err)
	}
	c.mu.Lock()
	c.cache[kubeContext] = topology{objects: list.Items, fetchedAt: time.Now()}
	c.mu.Unlock()
	return list.Items, nil
}

func aggregate(result *clusterUsage, objects []k8sObject, summaries []nodeSummary, allNamespaces bool) *clusterUsage {
	podMetrics := make(map[string]usage)
	for _, summary := range summaries {
		if summary.Node.CPU != nil {
			result.NodeCPU += summary.Node.CPU.UsageNanoCores
		}
		if summary.Node.Memory != nil {
			result.NodeMemory += summary.Node.Memory.WorkingSetBytes
		}
		for _, pod := range summary.Pods {
			value := usage{CPU: pod.CPU, Memory: pod.Memory}
			podMetrics[pod.PodRef.UID] = value
			podMetrics[pod.PodRef.Namespace+"/"+pod.PodRef.Name] = value
		}
	}

	replicaSetOwners := make(map[string]workloadKey)
	controllers := make(map[workloadKey]k8sObject)
	var pods []k8sObject
	for _, object := range objects {
		switch object.Kind {
		case "ReplicaSet":
			owner := controllerOwner(object.Metadata.OwnerReferences)
			if owner.Kind == "Deployment" {
				replicaSetOwners[object.Metadata.Namespace+"/"+object.Metadata.Name] = workloadKey{
					Namespace: object.Metadata.Namespace, Kind: owner.Kind, Name: owner.Name,
				}
			}
		case "Deployment", "StatefulSet", "DaemonSet", "Job":
			if includeNamespace(object.Metadata.Namespace, allNamespaces) {
				key := workloadKey{Namespace: object.Metadata.Namespace, Kind: object.Kind, Name: object.Metadata.Name}
				controllers[key] = object
			}
		case "Pod":
			if includeNamespace(object.Metadata.Namespace, allNamespaces) {
				pods = append(pods, object)
			}
		}
	}

	apps := make(map[string]map[string]*appUsage)
	ensureApp := func(key workloadKey) *appUsage {
		if apps[key.Namespace] == nil {
			apps[key.Namespace] = make(map[string]*appUsage)
		}
		family := collapseFamily(key.Name)
		app := apps[key.Namespace][family]
		if app == nil {
			app = &appUsage{Name: family, Kinds: make(map[string]bool)}
			apps[key.Namespace][family] = app
		}
		app.Kinds[key.Kind] = true
		return app
	}

	for key, object := range controllers {
		app := ensureApp(key)
		app.Workloads++
		switch object.Kind {
		case "Deployment", "StatefulSet":
			if object.Spec.Replicas != nil {
				app.Desired += *object.Spec.Replicas
			}
		case "DaemonSet":
			app.Desired++
		case "Job":
			app.Desired += object.Status.Active
		}
	}

	for _, pod := range pods {
		key := podWorkload(pod, replicaSetOwners)
		app := ensureApp(key)
		app.Pods++
		app.Containers += len(pod.Spec.Containers)
		if podReady(pod) {
			app.ReadyPods++
		}
		metric, ok := podMetrics[pod.Metadata.UID]
		if !ok {
			metric = podMetrics[pod.Metadata.Namespace+"/"+pod.Metadata.Name]
		}
		if metric.CPU != nil {
			app.CPU += metric.CPU.UsageNanoCores
		}
		if metric.Memory != nil {
			app.Memory += metric.Memory.WorkingSetBytes
		}
		for _, container := range pod.Spec.Containers {
			app.CPURequest += parseCPU(container.Resources.Requests["cpu"])
			app.MemoryRequest += parseBytes(container.Resources.Requests["memory"])
			app.CPULimit += parseCPU(container.Resources.Limits["cpu"])
			app.MemoryLimit += parseBytes(container.Resources.Limits["memory"])
		}
	}

	for namespace, appMap := range apps {
		ns := &namespaceUsage{Name: namespace}
		for _, app := range appMap {
			ns.Apps = append(ns.Apps, app)
			ns.CPU += app.CPU
			ns.Memory += app.Memory
			ns.Pods += app.Pods
			ns.ReadyPods += app.ReadyPods
			ns.Containers += app.Containers
			ns.CPURequest += app.CPURequest
			ns.MemoryRequest += app.MemoryRequest
			ns.CPULimit += app.CPULimit
			ns.MemoryLimit += app.MemoryLimit
		}
		sort.Slice(ns.Apps, func(i, j int) bool {
			if ns.Apps[i].Memory != ns.Apps[j].Memory {
				return ns.Apps[i].Memory > ns.Apps[j].Memory
			}
			return ns.Apps[i].Name < ns.Apps[j].Name
		})
		result.Namespaces = append(result.Namespaces, ns)
		result.AppCPU += ns.CPU
		result.AppMemory += ns.Memory
		result.Pods += ns.Pods
		result.ReadyPods += ns.ReadyPods
		result.Containers += ns.Containers
		result.CPURequest += ns.CPURequest
		result.MemoryRequest += ns.MemoryRequest
		result.CPULimit += ns.CPULimit
		result.MemoryLimit += ns.MemoryLimit
	}
	sort.Slice(result.Namespaces, func(i, j int) bool {
		if result.Namespaces[i].Memory != result.Namespaces[j].Memory {
			return result.Namespaces[i].Memory > result.Namespaces[j].Memory
		}
		return result.Namespaces[i].Name < result.Namespaces[j].Name
	})
	return result
}

func podWorkload(pod k8sObject, replicaSetOwners map[string]workloadKey) workloadKey {
	owner := controllerOwner(pod.Metadata.OwnerReferences)
	if owner.Kind == "ReplicaSet" {
		if deployment, ok := replicaSetOwners[pod.Metadata.Namespace+"/"+owner.Name]; ok {
			return deployment
		}
	}
	if owner.Name != "" {
		return workloadKey{Namespace: pod.Metadata.Namespace, Kind: owner.Kind, Name: owner.Name}
	}
	return workloadKey{Namespace: pod.Metadata.Namespace, Kind: "Pod", Name: pod.Metadata.Name}
}

func controllerOwner(owners []ownerReference) ownerReference {
	for _, owner := range owners {
		if owner.Controller {
			return owner
		}
	}
	if len(owners) > 0 {
		return owners[0]
	}
	return ownerReference{}
}

func podReady(pod k8sObject) bool {
	for _, value := range pod.Status.Conditions {
		if value.Type == "Ready" {
			return value.Status == "True"
		}
	}
	return false
}

func includeNamespace(namespace string, all bool) bool {
	if all {
		return true
	}
	switch namespace {
	case "kube-system", "kube-public", "kube-node-lease", "local-path-storage":
		return false
	default:
		return namespace != ""
	}
}

func collapseFamily(name string) string {
	match := slotFamilyPattern.FindStringSubmatch(name)
	if match == nil {
		return name
	}
	return match[1] + "*" + match[2]
}

func indexClusters(clusters []*clusterUsage) map[string]*clusterUsage {
	indexed := make(map[string]*clusterUsage, len(clusters))
	for _, cluster := range clusters {
		indexed[cluster.Context] = cluster
	}
	return indexed
}

func applyDeltas(current []*clusterUsage, previous map[string]*clusterUsage) {
	for _, cluster := range current {
		oldCluster := previous[cluster.Context]
		if oldCluster == nil || cluster.Error != nil || oldCluster.Error != nil {
			continue
		}
		cluster.MemoryDelta = int64(cluster.NodeMemory) - int64(oldCluster.NodeMemory)
		oldNamespaces := make(map[string]*namespaceUsage, len(oldCluster.Namespaces))
		for _, namespace := range oldCluster.Namespaces {
			oldNamespaces[namespace.Name] = namespace
		}
		for _, namespace := range cluster.Namespaces {
			oldNamespace := oldNamespaces[namespace.Name]
			if oldNamespace == nil {
				continue
			}
			namespace.MemoryDelta = int64(namespace.Memory) - int64(oldNamespace.Memory)
			oldApps := make(map[string]*appUsage, len(oldNamespace.Apps))
			for _, app := range oldNamespace.Apps {
				oldApps[app.Name] = app
			}
			for _, app := range namespace.Apps {
				if oldApp := oldApps[app.Name]; oldApp != nil {
					app.MemoryDelta = int64(app.Memory) - int64(oldApp.Memory)
				}
			}
		}
	}
}

func filterClusters(clusters []*clusterUsage, filter string) []*clusterUsage {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return clusters
	}
	var view []*clusterUsage
	for _, cluster := range clusters {
		if cluster.Error != nil {
			continue
		}
		var namespaces []*namespaceUsage
		for _, namespace := range cluster.Namespaces {
			if strings.Contains(strings.ToLower(namespace.Name), filter) {
				namespaces = append(namespaces, namespace)
				continue
			}
			var apps []*appUsage
			for _, app := range namespace.Apps {
				if strings.Contains(strings.ToLower(app.Name), filter) {
					apps = append(apps, app)
				}
			}
			if len(apps) == 0 {
				continue
			}
			narrowed := *namespace
			narrowed.Apps = apps
			namespaces = append(namespaces, &narrowed)
		}
		if len(namespaces) == 0 {
			continue
		}
		narrowed := *cluster
		narrowed.Namespaces = namespaces
		view = append(view, &narrowed)
	}
	return view
}

func sortClusters(clusters []*clusterUsage, sortKey string) {
	sort.SliceStable(clusters, func(i, j int) bool {
		if (clusters[i].Error == nil) != (clusters[j].Error == nil) {
			return clusters[i].Error == nil
		}
		return usageLess(clusters[i].NodeCPU, clusters[j].NodeCPU, clusters[i].NodeMemory, clusters[j].NodeMemory, clusters[i].Name, clusters[j].Name, sortKey)
	})
	for _, cluster := range clusters {
		sort.SliceStable(cluster.Namespaces, func(i, j int) bool {
			return usageLess(cluster.Namespaces[i].CPU, cluster.Namespaces[j].CPU, cluster.Namespaces[i].Memory, cluster.Namespaces[j].Memory, cluster.Namespaces[i].Name, cluster.Namespaces[j].Name, sortKey)
		})
		for _, namespace := range cluster.Namespaces {
			sort.SliceStable(namespace.Apps, func(i, j int) bool {
				return usageLess(namespace.Apps[i].CPU, namespace.Apps[j].CPU, namespace.Apps[i].Memory, namespace.Apps[j].Memory, namespace.Apps[i].Name, namespace.Apps[j].Name, sortKey)
			})
		}
	}
}

func usageLess(cpuA, cpuB, memoryA, memoryB uint64, nameA, nameB, sortKey string) bool {
	switch sortKey {
	case "cpu":
		if cpuA != cpuB {
			return cpuA > cpuB
		}
	case "name":
		if nameA != nameB {
			return nameA < nameB
		}
	}
	if memoryA != memoryB {
		return memoryA > memoryB
	}
	return nameA < nameB
}

func run(parent context.Context, timeout time.Duration, executable string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("timed out after %s", timeout)
	}
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, errors.New(message)
	}
	return out, nil
}

func keyboardInput() (<-chan byte, func()) {
	disabled := make(chan byte)
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return disabled, func() {}
	}
	saved, err := stty("-g")
	if err != nil {
		return disabled, func() {}
	}
	if _, err := stty("-icanon", "-echo", "min", "1", "time", "0"); err != nil {
		return disabled, func() {}
	}

	keys := make(chan byte, 8)
	go func() {
		buffer := make([]byte, 1)
		for {
			if _, err := os.Stdin.Read(buffer); err != nil {
				close(keys)
				return
			}
			keys <- buffer[0]
		}
	}()
	var once sync.Once
	restore := func() {
		once.Do(func() {
			_, _ = stty(strings.TrimSpace(string(saved)))
		})
	}
	return keys, restore
}

func stty(args ...string) ([]byte, error) {
	cmd := exec.Command("stty", args...)
	cmd.Stdin = os.Stdin
	return cmd.CombinedOutput()
}

const (
	minimumNameWidth  = 44
	cpuColumnWidth    = 15
	memoryColumnWidth = 18
	requestWidth      = 8
	limitWidth        = 8
	deltaWidth        = 8
	appMemoryWidth    = 9
	podsColumnWidth   = 9
	containersWidth   = 5
)

type displayRow struct {
	name       string
	nameColor  string
	background string
	cpu        string
	memory     string
	request    string
	limit      string
	delta      string
	appMemory  string
	pods       string
	podsColor  string
	containers string
}

func render(out io.Writer, clusters []*clusterUsage, opts options, elapsed time.Duration) {
	if !opts.once {
		fmt.Fprint(out, "\033[H\033[2J")
	}
	bold, dim, cyan, green, yellow, red, reset := colors(opts.noColor)
	totalBackground, clusterBackground, namespaceBackground, appBackground := backgrounds(opts.noColor)
	filterLabel := ""
	if strings.TrimSpace(opts.filter) != "" {
		filterLabel = "  filter " + opts.filter
	}
	fmt.Fprintf(out, "%sKIND APP TOP%s  %s%s%s  level %s%s%s  sort %s%s%s%s  %s[ / ] level  m/c/n sort%s  refresh %s  collected %s  q quit\n\n",
		bold, reset, dim, time.Now().Format("15:04:05"), reset,
		bold, opts.level.String(), reset, bold, opts.sortKey, reset, filterLabel, dim, reset, opts.interval, elapsed.Round(time.Millisecond))
	nameWidth := displayNameWidth(clusters)
	writeDisplayRow(out, nameWidth, displayRow{
		name: "RESOURCE", cpu: "CPU USED/CAP", memory: "MEM USED/CAP",
		request: "MEM REQ", limit: "MEM LIM", delta: "ΔMEM",
		appMemory: "APP MEM", pods: "PODS", containers: "CTRS", nameColor: bold,
	}, reset)
	fmt.Fprintln(out, dim+strings.Repeat("─", tableWidth(nameWidth))+reset)
	total := totalUsage(clusters)
	totalStatusColor := green
	if total.readyClusters < total.clusters || total.readyPods < total.pods {
		totalStatusColor = yellow
	}
	writeDisplayRow(out, nameWidth, displayRow{
		name:       fmt.Sprintf("TOTAL · %d/%d clusters", total.readyClusters, total.clusters),
		nameColor:  bold,
		background: totalBackground,
		cpu:        formatCPU(total.cpu) + " / " + formatCPU(total.cpuCapacity),
		memory:     formatBytes(total.memory) + " / " + formatBytes(total.memoryCapacity),
		request:    formatBytes(total.memoryRequest),
		limit:      formatBytes(total.memoryLimit),
		delta:      formatSignedBytes(total.memoryDelta),
		appMemory:  formatBytes(total.appMemory),
		pods:       fmt.Sprintf("%d/%d", total.readyPods, total.pods),
		podsColor:  totalStatusColor,
		containers: strconv.Itoa(total.containers),
	}, reset)
	if opts.level == levelTotal {
		return
	}
	fmt.Fprintln(out)
	for _, cluster := range clusters {
		if cluster.Error != nil {
			writeDisplayRow(out, nameWidth, displayRow{
				name: "● " + cluster.Name, nameColor: red + bold, background: clusterBackground,
				pods: "UNAVAILABLE", podsColor: red,
			}, reset)
			fmt.Fprintf(out, "%s    %s%s\n\n", dim, oneLine(cluster.Error.Error()), reset)
			continue
		}
		statusColor := green
		if cluster.ReadyPods < cluster.Pods {
			statusColor = yellow
		}
		writeDisplayRow(out, nameWidth, displayRow{
			name:       "● " + cluster.Name,
			nameColor:  statusColor + bold,
			background: clusterBackground,
			cpu:        formatCPU(cluster.NodeCPU) + " / " + formatCPU(cluster.CPUCapacity),
			memory:     formatBytes(cluster.NodeMemory) + " / " + formatBytes(cluster.MemoryCapacity),
			request:    formatBytes(cluster.MemoryRequest),
			limit:      formatBytes(cluster.MemoryLimit),
			delta:      formatSignedBytes(cluster.MemoryDelta),
			appMemory:  formatBytes(cluster.AppMemory),
			pods:       fmt.Sprintf("%d/%d", cluster.ReadyPods, cluster.Pods),
			podsColor:  statusColor,
			containers: strconv.Itoa(cluster.Containers),
		}, reset)
		if opts.level == levelClusters {
			fmt.Fprintln(out)
			continue
		}
		for namespaceIndex, namespace := range cluster.Namespaces {
			podsColor := green
			if namespace.ReadyPods < namespace.Pods {
				podsColor = yellow
			}
			writeDisplayRow(out, nameWidth, displayRow{
				name:       "  " + namespace.Name,
				nameColor:  cyan,
				background: namespaceBackground,
				cpu:        formatCPU(namespace.CPU),
				memory:     formatBytes(namespace.Memory),
				request:    formatBytes(namespace.MemoryRequest),
				limit:      formatBytes(namespace.MemoryLimit),
				delta:      formatSignedBytes(namespace.MemoryDelta),
				pods:       fmt.Sprintf("%d/%d", namespace.ReadyPods, namespace.Pods),
				podsColor:  podsColor,
				containers: strconv.Itoa(namespace.Containers),
			}, reset)
			if opts.level == levelNamespaces {
				continue
			}
			for appIndex, app := range namespace.Apps {
				name := app.Name
				if app.Workloads > 1 {
					name += fmt.Sprintf(" ×%d", app.Workloads)
				}
				state := fmt.Sprintf("%d/%d", app.ReadyPods, app.Pods)
				podsColor := green
				if app.ReadyPods < app.Pods {
					podsColor = yellow
				}
				if app.Pods == 0 && app.Desired == 0 {
					state = "halted"
					podsColor = dim
				}
				writeDisplayRow(out, nameWidth, displayRow{
					name:       "    " + name,
					background: appBackground[(namespaceIndex+appIndex)%len(appBackground)],
					cpu:        formatCPU(app.CPU),
					memory:     formatBytes(app.Memory),
					request:    formatBytes(app.MemoryRequest),
					limit:      formatBytes(app.MemoryLimit),
					delta:      formatSignedBytes(app.MemoryDelta),
					pods:       state,
					podsColor:  podsColor,
					containers: strconv.Itoa(app.Containers),
				}, reset)
			}
		}
		fmt.Fprintln(out)
	}
}

func displayNameWidth(clusters []*clusterUsage) int {
	width := minimumNameWidth
	for _, cluster := range clusters {
		width = max(width, len([]rune("● "+cluster.Name)))
		for _, namespace := range cluster.Namespaces {
			width = max(width, len([]rune("  "+namespace.Name)))
			for _, app := range namespace.Apps {
				name := "    " + app.Name
				if app.Workloads > 1 {
					name += fmt.Sprintf(" ×%d", app.Workloads)
				}
				width = max(width, len([]rune(name)))
			}
		}
	}
	return width
}

func writeDisplayRow(out io.Writer, nameWidth int, row displayRow, reset string) {
	name := padRight(row.name, nameWidth)
	cpu := padLeft(row.cpu, cpuColumnWidth)
	memory := padLeft(row.memory, memoryColumnWidth)
	request := padLeft(row.request, requestWidth)
	limit := padLeft(row.limit, limitWidth)
	delta := padLeft(row.delta, deltaWidth)
	appMemory := padLeft(row.appMemory, appMemoryWidth)
	pods := padLeft(row.pods, podsColumnWidth)
	containers := padLeft(row.containers, containersWidth)
	foregroundReset := ""
	if reset != "" {
		foregroundReset = "\033[39m"
	}
	fmt.Fprintf(out, "%s%s%s%s  %s  %s  %s  %s  %s  %s  %s%s%s  %s%s\n",
		row.background, row.nameColor, name, foregroundReset,
		cpu, memory, request, limit, delta, appMemory, row.podsColor, pods, foregroundReset, containers, reset)
}

type totals struct {
	clusters, readyClusters int
	cpu, cpuCapacity        uint64
	memory, memoryCapacity  uint64
	appMemory               uint64
	memoryRequest           uint64
	memoryLimit             uint64
	memoryDelta             int64
	pods, readyPods         int
	containers              int
}

func totalUsage(clusters []*clusterUsage) totals {
	result := totals{clusters: len(clusters)}
	for _, cluster := range clusters {
		if cluster.Error != nil {
			continue
		}
		result.readyClusters++
		result.cpu += cluster.NodeCPU
		result.memory += cluster.NodeMemory
		result.appMemory += cluster.AppMemory
		result.memoryRequest += cluster.MemoryRequest
		result.memoryLimit += cluster.MemoryLimit
		result.memoryDelta += cluster.MemoryDelta
		result.pods += cluster.Pods
		result.readyPods += cluster.ReadyPods
		result.containers += cluster.Containers
		// Local Kind clusters share the same container-runtime VM. Capacity is
		// therefore the largest cluster-visible host capacity, not their sum.
		result.cpuCapacity = max(result.cpuCapacity, cluster.CPUCapacity)
		result.memoryCapacity = max(result.memoryCapacity, cluster.MemoryCapacity)
	}
	return result
}

func tableWidth(nameWidth int) int {
	return nameWidth + cpuColumnWidth + memoryColumnWidth + requestWidth + limitWidth + deltaWidth + appMemoryWidth + podsColumnWidth + containersWidth + 16
}

func padLeft(value string, width int) string {
	padding := width - len([]rune(value))
	if padding < 0 {
		padding = 0
	}
	return strings.Repeat(" ", padding) + value
}

func padRight(value string, width int) string {
	padding := width - len([]rune(value))
	if padding < 0 {
		padding = 0
	}
	return value + strings.Repeat(" ", padding)
}

func colors(disabled bool) (string, string, string, string, string, string, string) {
	if disabled {
		return "", "", "", "", "", "", ""
	}
	return "\033[1m", "\033[2m", "\033[36m", "\033[32m", "\033[33m", "\033[31m", "\033[0m"
}

func backgrounds(disabled bool) (string, string, string, []string) {
	if disabled {
		return "", "", "", []string{"", ""}
	}
	return "\033[48;5;24m", "\033[48;5;236m", "\033[48;5;237m", []string{"\033[48;5;234m", "\033[48;5;235m"}
}

func formatCPU(nanoCores uint64) string {
	if nanoCores == 0 {
		return "0m"
	}
	milli := float64(nanoCores) / 1_000_000
	if milli >= 1000 {
		return fmt.Sprintf("%.2f", milli/1000)
	}
	return fmt.Sprintf("%.0fm", milli)
}

func formatSignedBytes(value int64) string {
	if value < 0 {
		return "-" + formatBytes(uint64(-value))
	}
	if value > 0 {
		return "+" + formatBytes(uint64(value))
	}
	return "0"
}

func formatBytes(value uint64) string {
	const (
		ki = uint64(1024)
		mi = 1024 * ki
		gi = 1024 * mi
	)
	switch {
	case value >= gi:
		return fmt.Sprintf("%.2fG", float64(value)/float64(gi))
	case value >= mi:
		return fmt.Sprintf("%.0fM", float64(value)/float64(mi))
	case value >= ki:
		return fmt.Sprintf("%.0fK", float64(value)/float64(ki))
	default:
		return fmt.Sprintf("%dB", value)
	}
}

func parseCPU(value string) uint64 {
	if strings.HasSuffix(value, "m") {
		milli, _ := strconv.ParseFloat(strings.TrimSuffix(value, "m"), 64)
		return uint64(milli * 1_000_000)
	}
	cores, _ := strconv.ParseFloat(value, 64)
	return uint64(cores * 1_000_000_000)
}

func parseBytes(value string) uint64 {
	units := []struct {
		suffix string
		factor float64
	}{
		{"Ei", 1 << 60}, {"Pi", 1 << 50}, {"Ti", 1 << 40}, {"Gi", 1 << 30}, {"Mi", 1 << 20}, {"Ki", 1 << 10},
		{"E", 1e18}, {"P", 1e15}, {"T", 1e12}, {"G", 1e9}, {"M", 1e6}, {"K", 1e3},
	}
	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			number, _ := strconv.ParseFloat(strings.TrimSuffix(value, unit.suffix), 64)
			return uint64(number * unit.factor)
		}
	}
	number, _ := strconv.ParseUint(value, 10, 64)
	return number
}

type jsonSnapshot struct {
	Sort     string        `json:"sort"`
	Filter   string        `json:"filter,omitempty"`
	Clusters []jsonCluster `json:"clusters"`
}

type jsonCluster struct {
	Name                 string          `json:"name"`
	Context              string          `json:"context"`
	Error                string          `json:"error,omitempty"`
	NodeCPUNanoCores     uint64          `json:"node_cpu_nanocores,omitempty"`
	NodeMemoryBytes      uint64          `json:"node_memory_bytes,omitempty"`
	CPUCapacityNanoCores uint64          `json:"cpu_capacity_nanocores,omitempty"`
	MemoryCapacityBytes  uint64          `json:"memory_capacity_bytes,omitempty"`
	MemoryRequestBytes   uint64          `json:"memory_request_bytes,omitempty"`
	MemoryLimitBytes     uint64          `json:"memory_limit_bytes,omitempty"`
	MemoryDeltaBytes     int64           `json:"memory_delta_bytes,omitempty"`
	Pods                 int             `json:"pods,omitempty"`
	ReadyPods            int             `json:"ready_pods,omitempty"`
	Namespaces           []jsonNamespace `json:"namespaces,omitempty"`
}

type jsonNamespace struct {
	Name               string    `json:"name"`
	CPUNanoCores       uint64    `json:"cpu_nanocores,omitempty"`
	MemoryBytes        uint64    `json:"memory_bytes,omitempty"`
	CPURequestNano     uint64    `json:"cpu_request_nanocores,omitempty"`
	MemoryRequestBytes uint64    `json:"memory_request_bytes,omitempty"`
	CPULimitNano       uint64    `json:"cpu_limit_nanocores,omitempty"`
	MemoryLimitBytes   uint64    `json:"memory_limit_bytes,omitempty"`
	MemoryDeltaBytes   int64     `json:"memory_delta_bytes,omitempty"`
	Pods               int       `json:"pods,omitempty"`
	ReadyPods          int       `json:"ready_pods,omitempty"`
	Apps               []jsonApp `json:"apps,omitempty"`
}

type jsonApp struct {
	Name               string `json:"name"`
	Workloads          int    `json:"workloads,omitempty"`
	CPUNanoCores       uint64 `json:"cpu_nanocores,omitempty"`
	MemoryBytes        uint64 `json:"memory_bytes,omitempty"`
	CPURequestNano     uint64 `json:"cpu_request_nanocores,omitempty"`
	MemoryRequestBytes uint64 `json:"memory_request_bytes,omitempty"`
	CPULimitNano       uint64 `json:"cpu_limit_nanocores,omitempty"`
	MemoryLimitBytes   uint64 `json:"memory_limit_bytes,omitempty"`
	MemoryDeltaBytes   int64  `json:"memory_delta_bytes,omitempty"`
	Pods               int    `json:"pods,omitempty"`
	ReadyPods          int    `json:"ready_pods,omitempty"`
	Desired            int64  `json:"desired,omitempty"`
}

func writeJSON(out io.Writer, clusters []*clusterUsage, opts options) error {
	snapshot := jsonSnapshot{Sort: opts.sortKey, Filter: opts.filter}
	for _, cluster := range clusters {
		item := jsonCluster{
			Name: cluster.Name, Context: cluster.Context,
			NodeCPUNanoCores: cluster.NodeCPU, NodeMemoryBytes: cluster.NodeMemory,
			CPUCapacityNanoCores: cluster.CPUCapacity, MemoryCapacityBytes: cluster.MemoryCapacity,
			MemoryRequestBytes: cluster.MemoryRequest, MemoryLimitBytes: cluster.MemoryLimit,
			MemoryDeltaBytes: cluster.MemoryDelta, Pods: cluster.Pods, ReadyPods: cluster.ReadyPods,
		}
		if cluster.Error != nil {
			item.Error = oneLine(cluster.Error.Error())
		}
		for _, namespace := range cluster.Namespaces {
			ns := jsonNamespace{
				Name: namespace.Name, CPUNanoCores: namespace.CPU, MemoryBytes: namespace.Memory,
				CPURequestNano: namespace.CPURequest, MemoryRequestBytes: namespace.MemoryRequest,
				CPULimitNano: namespace.CPULimit, MemoryLimitBytes: namespace.MemoryLimit,
				MemoryDeltaBytes: namespace.MemoryDelta, Pods: namespace.Pods, ReadyPods: namespace.ReadyPods,
			}
			for _, app := range namespace.Apps {
				ns.Apps = append(ns.Apps, jsonApp{
					Name: app.Name, Workloads: app.Workloads,
					CPUNanoCores: app.CPU, MemoryBytes: app.Memory,
					CPURequestNano: app.CPURequest, MemoryRequestBytes: app.MemoryRequest,
					CPULimitNano: app.CPULimit, MemoryLimitBytes: app.MemoryLimit,
					MemoryDeltaBytes: app.MemoryDelta, Pods: app.Pods, ReadyPods: app.ReadyPods,
					Desired: app.Desired,
				})
			}
			item.Namespaces = append(item.Namespaces, ns)
		}
		snapshot.Clusters = append(snapshot.Clusters, item)
	}
	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	_, err = out.Write(encoded)
	return err
}

func oneLine(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 110 {
		return value[:107] + "..."
	}
	return value
}
