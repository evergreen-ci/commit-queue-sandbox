package plank

// Build is the API model representing metadata of a Logkeeper build.
type Build struct {
	ID            string `json:"id"`
	Builder       string `json:"builder"`
	BuildNum      int    `json:"buildnum"`
	TaskID        string `json:"task_id"`
	TaskExecution int    `json:"execution"`
	Tests         []Test `json:"tests"`
}

// Test is the API model representing metadata of a Logkeeper test.
type Test struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	BuildID       string `json:"build_id"`
	TaskID        string `json:"task_id"`
	TaskExecution int    `json:"execution"`
	Phase         string `json:"phase"`
	Command       string `json:"command"`
}
