export namespace auth {
	
	export class State {
	    authenticated: boolean;
	    has_token: boolean;
	    offline: boolean;
	    message: string;
	    token_info?: cloudflare.TokenInfo;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.authenticated = source["authenticated"];
	        this.has_token = source["has_token"];
	        this.offline = source["offline"];
	        this.message = source["message"];
	        this.token_info = this.convertValues(source["token_info"], cloudflare.TokenInfo);
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

export namespace cloudflare {
	
	export class Permissions {
	    tunnel_edit: string;
	    zone_read: string;
	    dns_edit: string;
	
	    static createFrom(source: any = {}) {
	        return new Permissions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tunnel_edit = source["tunnel_edit"];
	        this.zone_read = source["zone_read"];
	        this.dns_edit = source["dns_edit"];
	    }
	}
	export class TokenInfo {
	    token_id: string;
	    account_id: string;
	    account_name: string;
	    permissions: Permissions;
	    first_zone_id: string;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new TokenInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token_id = source["token_id"];
	        this.account_id = source["account_id"];
	        this.account_name = source["account_name"];
	        this.permissions = this.convertValues(source["permissions"], Permissions);
	        this.first_zone_id = source["first_zone_id"];
	        this.warnings = source["warnings"];
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

