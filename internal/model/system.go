package model

// What the admin System page reports: execution engines, settings, the deployment.

// ExecutorStatus is what uspace reports about one execution engine.
type ExecutorStatus struct {
	Engine  string `json:"engine"`  // "docker" or "kubernetes"
	Active  bool   `json:"active"`  // the engine answers and jobs can be started
	Current bool   `json:"current"` // new jobs go to this engine
	Version string `json:"version,omitempty"`
	// Where jobs run: the kubernetes namespace or the docker network.
	Where     string `json:"where,omitempty"`
	Error     string `json:"error,omitempty"` // why it isn't active
	CheckedAt string `json:"checkedAt"`
}

// EnginesStatus is uspace's execution engines: each one's status, which
// takes new jobs (and who chose it when), and the jobs recorded per engine.
type EnginesStatus struct {
	Current string           `json:"current"`
	Engines []ExecutorStatus `json:"engines"`
	// ChosenBy/ChosenAt: the admin who switched to the current engine (empty
	// while it is the configured default, J_EXECUTOR)
	ChosenBy string `json:"chosenBy,omitempty"`
	ChosenAt string `json:"chosenAt,omitempty"`
	// JobsByEngine counts the recorded jobs per engine ("" for jobs that
	// never started, or ran before engines were recorded).
	JobsByEngine map[string]int `json:"jobsByEngine,omitempty"`
}

// Setting is a value an admin changed at runtime.
type Setting struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedBy string `json:"updatedBy"`
	UpdatedAt string `json:"updatedAt"`
}

// Deployment is where kuspace runs and how many instances each service has.
type Deployment struct {
	Platform  string            `json:"platform"`        // "kubernetes", "compose" or "" (neither)
	Where     string            `json:"where,omitempty"` // the namespace or compose project
	Services  []ServiceReplicas `json:"services"`
	Error     string            `json:"error,omitempty"`
	CheckedAt string            `json:"checkedAt"`
}

// ServiceReplicas is one workload: a Deployment or StatefulSet, or the
// containers of a compose service.
type ServiceReplicas struct {
	Name      string     `json:"name"`
	Kind      string     `json:"kind"` // "Deployment", "StatefulSet", "container"
	Desired   int        `json:"desired"`
	Ready     int        `json:"ready"`
	Instances []Instance `json:"instances"`
}

// Instance is one pod or container.
type Instance struct {
	Name      string `json:"name"`
	State     string `json:"state"` // running, pending, healthy, CrashLoopBackOff, exited, ...
	Ready     bool   `json:"ready"`
	Restarts  int    `json:"restarts"`
	StartedAt string `json:"startedAt,omitempty"`
	Node      string `json:"node,omitempty"`
	Image     string `json:"image,omitempty"`
}
