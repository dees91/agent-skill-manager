export namespace contextbudget {

	export class Usage {
	    skillCount: number;
	    requestedCharacters: number;
	    renderedCharacters: number;
	    estimatedTokens: number;
	    renderedTokens: number;
	    usedPercent: number;
	    shortenedDescriptions: number;
	    omittedSkills: number;
	    health: string;

	    static createFrom(source: any = {}) {
	        return new Usage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.skillCount = source["skillCount"];
	        this.requestedCharacters = source["requestedCharacters"];
	        this.renderedCharacters = source["renderedCharacters"];
	        this.estimatedTokens = source["estimatedTokens"];
	        this.renderedTokens = source["renderedTokens"];
	        this.usedPercent = source["usedPercent"];
	        this.shortenedDescriptions = source["shortenedDescriptions"];
	        this.omittedSkills = source["omittedSkills"];
	        this.health = source["health"];
	    }
	}
	export class ToolReport {
	    tool: string;
	    model: string;
	    contextWindowTokens: number;
	    contextWindowAssumed: boolean;
	    budgetFraction: number;
	    budgetCharacters: number;
	    budgetTokens: number;
	    budgetLabel: string;
	    accuracy: string;
	    coverage: string;
	    message: string;
	    current: Usage;
	    projected: Usage;
	    projectionChanged: boolean;

	    static createFrom(source: any = {}) {
	        return new ToolReport(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.model = source["model"];
	        this.contextWindowTokens = source["contextWindowTokens"];
	        this.contextWindowAssumed = source["contextWindowAssumed"];
	        this.budgetFraction = source["budgetFraction"];
	        this.budgetCharacters = source["budgetCharacters"];
	        this.budgetTokens = source["budgetTokens"];
	        this.budgetLabel = source["budgetLabel"];
	        this.accuracy = source["accuracy"];
	        this.coverage = source["coverage"];
	        this.message = source["message"];
	        this.current = this.convertValues(source["current"], Usage);
	        this.projected = this.convertValues(source["projected"], Usage);
	        this.projectionChanged = source["projectionChanged"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Reports {
	    claude: ToolReport;
	    codex: ToolReport;

	    static createFrom(source: any = {}) {
	        return new Reports(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.claude = this.convertValues(source["claude"], ToolReport);
	        this.codex = this.convertValues(source["codex"], ToolReport);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}


}

export namespace gui {

	export class ActionCounts {
	    changed: number;
	    removed: number;
	    skippedReadOnly: number;
	    skippedMissing: number;
	    skippedConflict: number;

	    static createFrom(source: any = {}) {
	        return new ActionCounts(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.changed = source["changed"];
	        this.removed = source["removed"];
	        this.skippedReadOnly = source["skippedReadOnly"];
	        this.skippedMissing = source["skippedMissing"];
	        this.skippedConflict = source["skippedConflict"];
	    }
	}
	export class SkillSetToolSummary {
	    tool: string;
	    appliedStatus: string;
	    effectiveStatus: string;
	    eligible: number;
	    on: number;
	    off: number;
	    effectiveOn: number;
	    effectiveOff: number;
	    pending: number;
	    missing: number;
	    readOnly: number;
	    conflict: number;

	    static createFrom(source: any = {}) {
	        return new SkillSetToolSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.appliedStatus = source["appliedStatus"];
	        this.effectiveStatus = source["effectiveStatus"];
	        this.eligible = source["eligible"];
	        this.on = source["on"];
	        this.off = source["off"];
	        this.effectiveOn = source["effectiveOn"];
	        this.effectiveOff = source["effectiveOff"];
	        this.pending = source["pending"];
	        this.missing = source["missing"];
	        this.readOnly = source["readOnly"];
	        this.conflict = source["conflict"];
	    }
	}
	export class SkillSetMemberCell {
	    tool: string;
	    state: string;
	    effectiveState: string;
	    pending?: string;
	    eligible: boolean;
	    reason?: string;

	    static createFrom(source: any = {}) {
	        return new SkillSetMemberCell(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.state = source["state"];
	        this.effectiveState = source["effectiveState"];
	        this.pending = source["pending"];
	        this.eligible = source["eligible"];
	        this.reason = source["reason"];
	    }
	}
	export class SkillSetMember {
	    name: string;
	    description: string;
	    group: string;
	    source: string;
	    available: boolean;
	    claude: SkillSetMemberCell;
	    codex: SkillSetMemberCell;

	    static createFrom(source: any = {}) {
	        return new SkillSetMember(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.group = source["group"];
	        this.source = source["source"];
	        this.available = source["available"];
	        this.claude = this.convertValues(source["claude"], SkillSetMemberCell);
	        this.codex = this.convertValues(source["codex"], SkillSetMemberCell);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SkillSet {
	    setId: string;
	    name: string;
	    description: string;
	    members: SkillSetMember[];
	    claude: SkillSetToolSummary;
	    codex: SkillSetToolSummary;
	    unavailable: number;
	    pending: number;
	    createdAt: string;
	    updatedAt: string;

	    static createFrom(source: any = {}) {
	        return new SkillSet(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.setId = source["setId"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.members = this.convertValues(source["members"], SkillSetMember);
	        this.claude = this.convertValues(source["claude"], SkillSetToolSummary);
	        this.codex = this.convertValues(source["codex"], SkillSetToolSummary);
	        this.unavailable = source["unavailable"];
	        this.pending = source["pending"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PendingChange {
	    tool: string;
	    skillName: string;
	    operation: string;

	    static createFrom(source: any = {}) {
	        return new PendingChange(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.skillName = source["skillName"];
	        this.operation = source["operation"];
	    }
	}
	export class ActionResult {
	    message: string;
	    counts: ActionCounts;
	    pending: PendingChange[];
	    contextBudgets: contextbudget.Reports;
	    skillSets: SkillSet[];
	    skillSetsWarning?: string;

	    static createFrom(source: any = {}) {
	        return new ActionResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.counts = this.convertValues(source["counts"], ActionCounts);
	        this.pending = this.convertValues(source["pending"], PendingChange);
	        this.contextBudgets = this.convertValues(source["contextBudgets"], contextbudget.Reports);
	        this.skillSets = this.convertValues(source["skillSets"], SkillSet);
	        this.skillSetsWarning = source["skillSetsWarning"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AppliedChange {
	    tool: string;
	    skillName: string;
	    operation: string;

	    static createFrom(source: any = {}) {
	        return new AppliedChange(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.skillName = source["skillName"];
	        this.operation = source["operation"];
	    }
	}
	export class ApplyFailure {
	    stage: string;
	    tool?: string;
	    skillName?: string;
	    operation?: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new ApplyFailure(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stage = source["stage"];
	        this.tool = source["tool"];
	        this.skillName = source["skillName"];
	        this.operation = source["operation"];
	        this.message = source["message"];
	    }
	}
	export class ConflictSummary {
	    tool: string;
	    skillName: string;
	    group: string;
	    blockerPath: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new ConflictSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.skillName = source["skillName"];
	        this.group = source["group"];
	        this.blockerPath = source["blockerPath"];
	        this.message = source["message"];
	    }
	}
	export class DashboardStats {
	    managedSkills: number;
	    readOnlySkills: number;
	    claude: StateCounts;
	    codex: StateCounts;
	    conflictCells: number;

	    static createFrom(source: any = {}) {
	        return new DashboardStats(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.managedSkills = source["managedSkills"];
	        this.readOnlySkills = source["readOnlySkills"];
	        this.claude = this.convertValues(source["claude"], StateCounts);
	        this.codex = this.convertValues(source["codex"], StateCounts);
	        this.conflictCells = source["conflictCells"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ManagedSource {
	    sourceId: string;
	    kind: string;
	    group: string;
	    location: string;
	    skillCount: number;
	    claudeCount: number;
	    codexCount: number;
	    installedAt: string;
	    commit?: string;
	    canUpdate: boolean;
	    updateMode: string;
	    updateHint: string;

	    static createFrom(source: any = {}) {
	        return new ManagedSource(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceId = source["sourceId"];
	        this.kind = source["kind"];
	        this.group = source["group"];
	        this.location = source["location"];
	        this.skillCount = source["skillCount"];
	        this.claudeCount = source["claudeCount"];
	        this.codexCount = source["codexCount"];
	        this.installedAt = source["installedAt"];
	        this.commit = source["commit"];
	        this.canUpdate = source["canUpdate"];
	        this.updateMode = source["updateMode"];
	        this.updateHint = source["updateHint"];
	    }
	}
	export class StateCounts {
	    on: number;
	    off: number;
	    conflict: number;
	    readOnly: number;

	    static createFrom(source: any = {}) {
	        return new StateCounts(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.on = source["on"];
	        this.off = source["off"];
	        this.conflict = source["conflict"];
	        this.readOnly = source["readOnly"];
	    }
	}
	export class GroupSummary {
	    group: string;
	    rows: number;
	    claude: StateCounts;
	    codex: StateCounts;
	    sources: string[];

	    static createFrom(source: any = {}) {
	        return new GroupSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.group = source["group"];
	        this.rows = source["rows"];
	        this.claude = this.convertValues(source["claude"], StateCounts);
	        this.codex = this.convertValues(source["codex"], StateCounts);
	        this.sources = source["sources"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Conflict {
	    originalPath: string;
	    disabledPath: string;
	    blockerPath: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new Conflict(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.originalPath = source["originalPath"];
	        this.disabledPath = source["disabledPath"];
	        this.blockerPath = source["blockerPath"];
	        this.message = source["message"];
	    }
	}
	export class SkillCell {
	    tool: string;
	    name: string;
	    displayName: string;
	    description: string;
	    state: string;
	    effectiveState: string;
	    pending?: string;
	    source: string;
	    group: string;
	    entryType: string;
	    activePath: string;
	    disabledPath: string;
	    skillFilePath: string;
	    symlinkTarget: string;
	    repoOrigin: string;
	    repoCommit: string;
	    readOnly: boolean;
	    conflict?: Conflict;

	    static createFrom(source: any = {}) {
	        return new SkillCell(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.description = source["description"];
	        this.state = source["state"];
	        this.effectiveState = source["effectiveState"];
	        this.pending = source["pending"];
	        this.source = source["source"];
	        this.group = source["group"];
	        this.entryType = source["entryType"];
	        this.activePath = source["activePath"];
	        this.disabledPath = source["disabledPath"];
	        this.skillFilePath = source["skillFilePath"];
	        this.symlinkTarget = source["symlinkTarget"];
	        this.repoOrigin = source["repoOrigin"];
	        this.repoCommit = source["repoCommit"];
	        this.readOnly = source["readOnly"];
	        this.conflict = this.convertValues(source["conflict"], Conflict);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SkillRow {
	    name: string;
	    description: string;
	    source: string;
	    group: string;
	    favorite: boolean;
	    claude?: SkillCell;
	    codex?: SkillCell;

	    static createFrom(source: any = {}) {
	        return new SkillRow(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.source = source["source"];
	        this.group = source["group"];
	        this.favorite = source["favorite"];
	        this.claude = this.convertValues(source["claude"], SkillCell);
	        this.codex = this.convertValues(source["codex"], SkillCell);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Snapshot {
	    rows: SkillRow[];
	    skillSets: SkillSet[];
	    skillSetsWarning?: string;
	    favoritesWarning?: string;
	    groups: GroupSummary[];
	    sources: string[];
	    managedSources: ManagedSource[];
	    stats: DashboardStats;
	    conflicts: ConflictSummary[];
	    contextBudgets: contextbudget.Reports;
	    pending: PendingChange[];
	    includeReadOnly: boolean;
	    scannedAt: string;

	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rows = this.convertValues(source["rows"], SkillRow);
	        this.skillSets = this.convertValues(source["skillSets"], SkillSet);
	        this.skillSetsWarning = source["skillSetsWarning"];
	        this.favoritesWarning = source["favoritesWarning"];
	        this.groups = this.convertValues(source["groups"], GroupSummary);
	        this.sources = source["sources"];
	        this.managedSources = this.convertValues(source["managedSources"], ManagedSource);
	        this.stats = this.convertValues(source["stats"], DashboardStats);
	        this.conflicts = this.convertValues(source["conflicts"], ConflictSummary);
	        this.contextBudgets = this.convertValues(source["contextBudgets"], contextbudget.Reports);
	        this.pending = this.convertValues(source["pending"], PendingChange);
	        this.includeReadOnly = source["includeReadOnly"];
	        this.scannedAt = source["scannedAt"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ApplyResult {
	    completed: AppliedChange[];
	    failure?: ApplyFailure;
	    message: string;
	    snapshot: Snapshot;

	    static createFrom(source: any = {}) {
	        return new ApplyResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.completed = this.convertValues(source["completed"], AppliedChange);
	        this.failure = this.convertValues(source["failure"], ApplyFailure);
	        this.message = source["message"];
	        this.snapshot = this.convertValues(source["snapshot"], Snapshot);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}



	export class FavoriteMutationResult {
	    message: string;
	    favorites: string[];
	    warning?: string;

	    static createFrom(source: any = {}) {
	        return new FavoriteMutationResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.favorites = source["favorites"];
	        this.warning = source["warning"];
	    }
	}

	export class InstallCandidateCell {
	    tool: string;
	    status: string;
	    message?: string;

	    static createFrom(source: any = {}) {
	        return new InstallCandidateCell(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class InstallCandidate {
	    name: string;
	    relativePath: string;
	    claude: InstallCandidateCell;
	    codex: InstallCandidateCell;

	    static createFrom(source: any = {}) {
	        return new InstallCandidate(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.relativePath = source["relativePath"];
	        this.claude = this.convertValues(source["claude"], InstallCandidateCell);
	        this.codex = this.convertValues(source["codex"], InstallCandidateCell);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

	export class InstallCellRequest {
	    skillName: string;
	    tool: string;

	    static createFrom(source: any = {}) {
	        return new InstallCellRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.skillName = source["skillName"];
	        this.tool = source["tool"];
	    }
	}
	export class InstallConflict {
	    skillName: string;
	    tool: string;
	    reason: string;
	    path?: string;

	    static createFrom(source: any = {}) {
	        return new InstallConflict(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.skillName = source["skillName"];
	        this.tool = source["tool"];
	        this.reason = source["reason"];
	        this.path = source["path"];
	    }
	}
	export class InstallDraft {
	    draftId: string;
	    kind: string;
	    group: string;
	    location: string;
	    candidates: InstallCandidate[];
	    cloned: boolean;
	    reused: boolean;
	    retainedClone: boolean;
	    cancelled: boolean;

	    static createFrom(source: any = {}) {
	        return new InstallDraft(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.draftId = source["draftId"];
	        this.kind = source["kind"];
	        this.group = source["group"];
	        this.location = source["location"];
	        this.candidates = this.convertValues(source["candidates"], InstallCandidate);
	        this.cloned = source["cloned"];
	        this.reused = source["reused"];
	        this.retainedClone = source["retainedClone"];
	        this.cancelled = source["cancelled"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InstallReview {
	    reviewId?: string;
	    draftId: string;
	    group: string;
	    selections: InstallCellRequest[];
	    createCount: number;
	    alreadyOnCount: number;
	    alreadyOffCount: number;
	    conflicts: InstallConflict[];
	    ready: boolean;

	    static createFrom(source: any = {}) {
	        return new InstallReview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reviewId = source["reviewId"];
	        this.draftId = source["draftId"];
	        this.group = source["group"];
	        this.selections = this.convertValues(source["selections"], InstallCellRequest);
	        this.createCount = source["createCount"];
	        this.alreadyOnCount = source["alreadyOnCount"];
	        this.alreadyOffCount = source["alreadyOffCount"];
	        this.conflicts = this.convertValues(source["conflicts"], InstallConflict);
	        this.ready = source["ready"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}





	export class SkillSetImpact {
	    setId: string;
	    name: string;
	    skills: string[];

	    static createFrom(source: any = {}) {
	        return new SkillSetImpact(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.setId = source["setId"];
	        this.name = source["name"];
	        this.skills = source["skills"];
	    }
	}


	export class SkillSetMutationResult {
	    message: string;
	    skillSets: SkillSet[];
	    warning?: string;

	    static createFrom(source: any = {}) {
	        return new SkillSetMutationResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.skillSets = this.convertValues(source["skillSets"], SkillSet);
	        this.warning = source["warning"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SkillSetTogglePreview {
	    setId: string;
	    name: string;
	    tools: string[];
	    direction: string;
	    eligible: number;
	    counts: ActionCounts;

	    static createFrom(source: any = {}) {
	        return new SkillSetTogglePreview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.setId = source["setId"];
	        this.name = source["name"];
	        this.tools = source["tools"];
	        this.direction = source["direction"];
	        this.eligible = source["eligible"];
	        this.counts = this.convertValues(source["counts"], ActionCounts);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}


	export class SourceMutationFailure {
	    stage: string;
	    group?: string;
	    message: string;
	    rolledBack?: number;
	    cleanupPending?: string;

	    static createFrom(source: any = {}) {
	        return new SourceMutationFailure(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stage = source["stage"];
	        this.group = source["group"];
	        this.message = source["message"];
	        this.rolledBack = source["rolledBack"];
	        this.cleanupPending = source["cleanupPending"];
	    }
	}
	export class SourceMutationItem {
	    sourceId: string;
	    group: string;
	    status: string;
	    before?: string;
	    after?: string;

	    static createFrom(source: any = {}) {
	        return new SourceMutationItem(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceId = source["sourceId"];
	        this.group = source["group"];
	        this.status = source["status"];
	        this.before = source["before"];
	        this.after = source["after"];
	    }
	}
	export class SourceMutationResult {
	    message: string;
	    completed: SourceMutationItem[];
	    failure?: SourceMutationFailure;
	    createdLinks?: number;
	    alreadyInstalled?: number;
	    removedActive?: number;
	    removedDisabled?: number;
	    snapshot: Snapshot;

	    static createFrom(source: any = {}) {
	        return new SourceMutationResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.completed = this.convertValues(source["completed"], SourceMutationItem);
	        this.failure = this.convertValues(source["failure"], SourceMutationFailure);
	        this.createdLinks = source["createdLinks"];
	        this.alreadyInstalled = source["alreadyInstalled"];
	        this.removedActive = source["removedActive"];
	        this.removedDisabled = source["removedDisabled"];
	        this.snapshot = this.convertValues(source["snapshot"], Snapshot);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

	export class UninstallPreview {
	    sourceId: string;
	    kind: string;
	    group: string;
	    location: string;
	    activeLinks: number;
	    disabledLinks: number;
	    removesCheckout: boolean;
	    preservesSource: boolean;
	    affectedSkillSets: SkillSetImpact[];
	    skillSetImpactWarning?: string;
	    affectedFavorites: string[];
	    favoriteImpactWarning?: string;

	    static createFrom(source: any = {}) {
	        return new UninstallPreview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceId = source["sourceId"];
	        this.kind = source["kind"];
	        this.group = source["group"];
	        this.location = source["location"];
	        this.activeLinks = source["activeLinks"];
	        this.disabledLinks = source["disabledLinks"];
	        this.removesCheckout = source["removesCheckout"];
	        this.preservesSource = source["preservesSource"];
	        this.affectedSkillSets = this.convertValues(source["affectedSkillSets"], SkillSetImpact);
	        this.skillSetImpactWarning = source["skillSetImpactWarning"];
	        this.affectedFavorites = source["affectedFavorites"];
	        this.favoriteImpactWarning = source["favoriteImpactWarning"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}
