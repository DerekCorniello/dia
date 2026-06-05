export namespace wailsapp {
	
	export class AppInfo {
	    type: string;
	    cmd: string;
	    args: string;
	    url?: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.cmd = source["cmd"];
	        this.args = source["args"];
	        this.url = source["url"];
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
	export class ProcessInfo {
	    type: string;
	    cmd: string;
	    pid: number;
	    status: string;
	    err?: string;
	
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
	    }
	}
	export class InstanceInfo {
	    id: string;
	    workspace: string;
	    workspace_path: string;
	    started_at: string;
	    status: string;
	    apps: ProcessInfo[];
	
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
	export class WorkspaceDetail {
	    name: string;
	    apps: number;
	    source: string;
	    path: string;
	    running: boolean;
	    app_details: AppInfo[];
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.apps = source["apps"];
	        this.source = source["source"];
	        this.path = source["path"];
	        this.running = source["running"];
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
	export class WorkspaceInfo {
	    name: string;
	    apps: number;
	    source: string;
	    path: string;
	    running: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.apps = source["apps"];
	        this.source = source["source"];
	        this.path = source["path"];
	        this.running = source["running"];
	    }
	}

}

