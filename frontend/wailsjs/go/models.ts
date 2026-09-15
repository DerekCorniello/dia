export namespace state {
	
	export class RecentEntry {
	    name: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new RecentEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.count = source["count"];
	    }
	}

}

export namespace wailsapp {
	
	export class AppEditor {
	    label: string;
	    type: string;
	    cmd: string;
	    cwd: string;
	    url: string;
	    browser: string;
	    urls: string[];
	    newWindow: boolean;
	    env: Record<string, string>;
	    args: string[];
	    termCmd: string;
	
	    static createFrom(source: any = {}) {
	        return new AppEditor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.type = source["type"];
	        this.cmd = source["cmd"];
	        this.cwd = source["cwd"];
	        this.url = source["url"];
	        this.browser = source["browser"];
	        this.urls = source["urls"];
	        this.newWindow = source["newWindow"];
	        this.env = source["env"];
	        this.args = source["args"];
	        this.termCmd = source["termCmd"];
	    }
	}
	export class AppInfo {
	    label?: string;
	    type: string;
	    cmd: string;
	    args: string;
	    url?: string;
	    browser?: string;
	    urls?: string[];
	    newWindow?: boolean;
	    cwd?: string;
	    env?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.type = source["type"];
	        this.cmd = source["cmd"];
	        this.args = source["args"];
	        this.url = source["url"];
	        this.browser = source["browser"];
	        this.urls = source["urls"];
	        this.newWindow = source["newWindow"];
	        this.cwd = source["cwd"];
	        this.env = source["env"];
	    }
	}
	export class FieldDescriptor {
	    name: string;
	    label: string;
	    type: string;
	    required?: boolean;
	    sensitive?: boolean;
	    help?: string;
	
	    static createFrom(source: any = {}) {
	        return new FieldDescriptor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.label = source["label"];
	        this.type = source["type"];
	        this.required = source["required"];
	        this.sensitive = source["sensitive"];
	        this.help = source["help"];
	    }
	}
	export class AppTypeDescriptor {
	    type: string;
	    label: string;
	    description: string;
	    fields: FieldDescriptor[];
	    summary?: string;
	
	    static createFrom(source: any = {}) {
	        return new AppTypeDescriptor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.label = source["label"];
	        this.description = source["description"];
	        this.fields = this.convertValues(source["fields"], FieldDescriptor);
	        this.summary = source["summary"];
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
	export class CapabilityInfo {
	    name: string;
	    mutating: boolean;
	    granted: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CapabilityInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.mutating = source["mutating"];
	        this.granted = source["granted"];
	    }
	}
	export class CheckInfo {
	    name: string;
	    status: string;
	    detail?: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.detail = source["detail"];
	    }
	}
	export class CustomThemeInfo {
	    name: string;
	    color_scheme: string;
	    colors: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new CustomThemeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.color_scheme = source["color_scheme"];
	        this.colors = source["colors"];
	    }
	}
	export class DaemonInfo {
	    reachable: boolean;
	    version?: string;
	    protocol?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new DaemonInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reachable = source["reachable"];
	        this.version = source["version"];
	        this.protocol = source["protocol"];
	        this.error = source["error"];
	    }
	}
	export class DetectedTool {
	    label: string;
	    command: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new DetectedTool(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.command = source["command"];
	        this.url = source["url"];
	    }
	}
	
	export class HooksEditor {
	    preStart?: string[];
	    postStart?: string[];
	    preStop?: string[];
	    postStop?: string[];
	
	    static createFrom(source: any = {}) {
	        return new HooksEditor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.preStart = source["preStart"];
	        this.postStart = source["postStart"];
	        this.preStop = source["preStop"];
	        this.postStop = source["postStop"];
	    }
	}
	export class ProcessInfo {
	    type: string;
	    cmd: string;
	    pid: number;
	    status: string;
	    err?: string;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProcessInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.cmd = source["cmd"];
	        this.pid = source["pid"];
	        this.status = source["status"];
	        this.err = source["err"];
	        this.note = source["note"];
	    }
	}
	export class InstanceInfo {
	    id: string;
	    workspace: string;
	    workspace_path: string;
	    started_at: string;
	    status: string;
	    apps: ProcessInfo[];
	    plugins?: string[];
	
	    static createFrom(source: any = {}) {
	        return new InstanceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspace = source["workspace"];
	        this.workspace_path = source["workspace_path"];
	        this.started_at = source["started_at"];
	        this.status = source["status"];
	        this.apps = this.convertValues(source["apps"], ProcessInfo);
	        this.plugins = source["plugins"];
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
	export class PathsInfo {
	    global_config_dir: string;
	    state_dir: string;
	    state_file: string;
	
	    static createFrom(source: any = {}) {
	        return new PathsInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.global_config_dir = source["global_config_dir"];
	        this.state_dir = source["state_dir"];
	        this.state_file = source["state_file"];
	    }
	}
	export class PluginActionDef {
	    id: string;
	    label: string;
	    confirm: boolean;
	    capability: string;
	
	    static createFrom(source: any = {}) {
	        return new PluginActionDef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.confirm = source["confirm"];
	        this.capability = source["capability"];
	    }
	}
	export class PluginCapabilityInfo {
	    requested: CapabilityInfo[];
	    granted: string[];
	
	    static createFrom(source: any = {}) {
	        return new PluginCapabilityInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requested = this.convertValues(source["requested"], CapabilityInfo);
	        this.granted = source["granted"];
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
	export class PluginUIColumn {
	    key: string;
	    label: string;
	    format?: string;
	
	    static createFrom(source: any = {}) {
	        return new PluginUIColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.format = source["format"];
	    }
	}
	export class PluginUIInfo {
	    type: string;
	    title: string;
	    refreshable: boolean;
	    columns?: PluginUIColumn[];
	
	    static createFrom(source: any = {}) {
	        return new PluginUIInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.title = source["title"];
	        this.refreshable = source["refreshable"];
	        this.columns = this.convertValues(source["columns"], PluginUIColumn);
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
	export class PluginInfo {
	    id: string;
	    name: string;
	    version: string;
	    description: string;
	    longDescription?: string;
	    author: string;
	    source: string;
	    dir: string;
	    status: string;
	    lastError?: string;
	    grantedCapabilities: string[];
	    ui: PluginUIInfo;
	    actions: PluginActionDef[];
	    configSchema?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new PluginInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.longDescription = source["longDescription"];
	        this.author = source["author"];
	        this.source = source["source"];
	        this.dir = source["dir"];
	        this.status = source["status"];
	        this.lastError = source["lastError"];
	        this.grantedCapabilities = source["grantedCapabilities"];
	        this.ui = this.convertValues(source["ui"], PluginUIInfo);
	        this.actions = this.convertValues(source["actions"], PluginActionDef);
	        this.configSchema = source["configSchema"];
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
	export class PluginPathsInfo {
	    globalDir: string;
	    localDir?: string;
	
	    static createFrom(source: any = {}) {
	        return new PluginPathsInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.globalDir = source["globalDir"];
	        this.localDir = source["localDir"];
	    }
	}
	export class PluginRefEditor {
	    id: string;
	    config: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new PluginRefEditor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.config = source["config"];
	    }
	}
	export class PluginSourceInfo {
	    id: string;
	    name: string;
	    version: string;
	    description: string;
	    author: string;
	    requested: CapabilityInfo[];
	    appTypes: string[];
	    isGit: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PluginSourceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.author = source["author"];
	        this.requested = this.convertValues(source["requested"], CapabilityInfo);
	        this.appTypes = source["appTypes"];
	        this.isGit = source["isGit"];
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
	
	
	
	export class ReconcileInfo {
	    reconciled: number;
	    remaining: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new ReconcileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reconciled = source["reconciled"];
	        this.remaining = source["remaining"];
	        this.total = source["total"];
	    }
	}
	export class ToolCategory {
	    name: string;
	    tools: DetectedTool[];
	
	    static createFrom(source: any = {}) {
	        return new ToolCategory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.tools = this.convertValues(source["tools"], DetectedTool);
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
	export class WorkspaceDetail {
	    name: string;
	    description?: string;
	    apps: number;
	    source: string;
	    path: string;
	    running: boolean;
	    lastStatus?: string;
	    lastError?: string;
	    plugins?: string[];
	    pluginCount?: number;
	    useCount?: number;
	    app_details: AppInfo[];
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.apps = source["apps"];
	        this.source = source["source"];
	        this.path = source["path"];
	        this.running = source["running"];
	        this.lastStatus = source["lastStatus"];
	        this.lastError = source["lastError"];
	        this.plugins = source["plugins"];
	        this.pluginCount = source["pluginCount"];
	        this.useCount = source["useCount"];
	        this.app_details = this.convertValues(source["app_details"], AppInfo);
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
	export class WorkspaceEditor {
	    originalName: string;
	    originalPath: string;
	    name: string;
	    description: string;
	    defaultCwd: string;
	    apps: AppEditor[];
	    plugins: PluginRefEditor[];
	    hooks?: HooksEditor;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceEditor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.originalName = source["originalName"];
	        this.originalPath = source["originalPath"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.defaultCwd = source["defaultCwd"];
	        this.apps = this.convertValues(source["apps"], AppEditor);
	        this.plugins = this.convertValues(source["plugins"], PluginRefEditor);
	        this.hooks = this.convertValues(source["hooks"], HooksEditor);
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
	export class WorkspaceInfo {
	    name: string;
	    description?: string;
	    apps: number;
	    source: string;
	    path: string;
	    running: boolean;
	    lastStatus?: string;
	    lastError?: string;
	    plugins?: string[];
	    pluginCount?: number;
	    useCount?: number;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.apps = source["apps"];
	        this.source = source["source"];
	        this.path = source["path"];
	        this.running = source["running"];
	        this.lastStatus = source["lastStatus"];
	        this.lastError = source["lastError"];
	        this.plugins = source["plugins"];
	        this.pluginCount = source["pluginCount"];
	        this.useCount = source["useCount"];
	    }
	}

}

