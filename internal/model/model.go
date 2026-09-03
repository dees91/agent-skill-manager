package model

// Tool identifies an agent tool with a user skill directory managed by the MVP.
type Tool string

const (
	ToolClaude Tool = "claude"
	ToolCodex  Tool = "codex"
	ToolMuse   Tool = "muse"
	ToolGrok   Tool = "grok"
)

func (t Tool) String() string {
	return string(t)
}

// ParseTool converts a CLI/UI tool name into a supported Tool.
func ParseTool(value string) (Tool, bool) {
	switch Tool(value) {
	case ToolClaude:
		return ToolClaude, true
	case ToolCodex:
		return ToolCodex, true
	case ToolMuse:
		return ToolMuse, true
	case ToolGrok:
		return ToolGrok, true
	default:
		return "", false
	}
}

// Tools returns the deterministic order used for tool-specific operations.
func Tools() []Tool {
	return []Tool{ToolClaude, ToolCodex, ToolMuse, ToolGrok}
}

// SkillState represents the state shown for one tool-specific skill cell.
type SkillState string

const (
	SkillStateOn       SkillState = "ON"
	SkillStateOff      SkillState = "OFF"
	SkillStateReadOnly SkillState = "RO"
	SkillStateMissing  SkillState = "-"
	SkillStatePending  SkillState = "PENDING"
	SkillStateConflict SkillState = "CONFLICT"
)

func (s SkillState) String() string {
	return string(s)
}

// SourceLabel classifies where a skill entry appears to come from.
type SourceLabel string

const (
	SourceSymlinkRepo  SourceLabel = "symlink repo"
	SourceSkillsCLI    SourceLabel = "Skills CLI"
	SourceLocal        SourceLabel = "local"
	SourceLocalPath    SourceLabel = "local path"
	SourceCodexSystem  SourceLabel = "Codex system"
	SourceClaudePlugin SourceLabel = "Claude plugin"
	SourceUnknown      SourceLabel = "unknown"
)

func (s SourceLabel) String() string {
	return string(s)
}

// GroupLabel identifies the source package, repository, or collection for a skill.
type GroupLabel string

const (
	GroupSkillsCLI    GroupLabel = "Skills CLI"
	GroupLocal        GroupLabel = "local"
	GroupCodexSystem  GroupLabel = "Codex system"
	GroupClaudePlugin GroupLabel = "Claude plugin"
	GroupUnknown      GroupLabel = "unknown"
)

func (g GroupLabel) String() string {
	return string(g)
}

// EntryType records whether the managed entry itself is a symlink or directory.
type EntryType string

const (
	EntryTypeSymlink EntryType = "symlink"
	EntryTypeDir     EntryType = "dir"
	EntryTypeUnknown EntryType = "unknown"
)

func (e EntryType) String() string {
	return string(e)
}

// OperationKind describes a reversible skill toggle operation.
type OperationKind string

const (
	OperationEnable  OperationKind = "enable"
	OperationDisable OperationKind = "disable"
)

func (o OperationKind) String() string {
	return string(o)
}

// Conflict describes a blocked restore path for an otherwise disabled skill.
type Conflict struct {
	OriginalPath string
	DisabledPath string
	BlockerPath  string
	Message      string
}

// ToolSkill is the tool-specific view of a skill entry.
type ToolSkill struct {
	Tool          Tool
	Name          string
	DisplayName   string
	Description   string
	State         SkillState
	Source        SourceLabel
	Group         GroupLabel
	EntryType     EntryType
	ActivePath    string
	DisabledPath  string
	SkillFilePath string
	SymlinkTarget string
	RepoOrigin    string
	RepoCommit    string
	ReadOnly      bool
	Pending       *OperationKind
	Conflict      *Conflict
}

// Toggleable reports whether this cell can produce enable/disable operations.
func (s ToolSkill) Toggleable() bool {
	return !s.ReadOnly && s.State != SkillStateReadOnly && s.State != SkillStateMissing
}

// SkillRow groups the Claude, Codex, Muse, and Grok cells for one skill name.
type SkillRow struct {
	Name        string
	Description string
	Source      SourceLabel
	Group       GroupLabel
	Claude      *ToolSkill
	Codex       *ToolSkill
	Muse        *ToolSkill
	Grok        *ToolSkill
}

// PlannedOperation is produced by planners and consumed by CLI/TUI executors.
type PlannedOperation struct {
	Kind          OperationKind
	Tool          Tool
	SkillName     string
	FromPath      string
	ToPath        string
	EntryType     EntryType
	SymlinkTarget string
	Source        SourceLabel
	Group         GroupLabel
}

// ToolStateCounts summarizes existing cells for one tool.
type ToolStateCounts struct {
	On       int
	Off      int
	Conflict int
	ReadOnly int
}

// GroupSummary summarizes rows that belong to one group.
type GroupSummary struct {
	Group      GroupLabel
	Rows       int
	Claude     ToolStateCounts
	Codex      ToolStateCounts
	Muse       ToolStateCounts
	Grok       ToolStateCounts
	Sources    []SourceLabel
	SourceText string
}
