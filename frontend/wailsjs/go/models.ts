export namespace kite {
	
	export class CredentialsInfo {
	    configured: boolean;
	    apiKeyMasked: string;
	    port: number;
	    redirectUrl: string;
	    secretBackend: string;
	
	    static createFrom(source: any = {}) {
	        return new CredentialsInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	        this.apiKeyMasked = source["apiKeyMasked"];
	        this.port = source["port"];
	        this.redirectUrl = source["redirectUrl"];
	        this.secretBackend = source["secretBackend"];
	    }
	}
	export class LoginStart {
	    loginUrl: string;
	    redirectUrl: string;
	    listenerOk: boolean;
	    listenerError: string;
	
	    static createFrom(source: any = {}) {
	        return new LoginStart(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loginUrl = source["loginUrl"];
	        this.redirectUrl = source["redirectUrl"];
	        this.listenerOk = source["listenerOk"];
	        this.listenerError = source["listenerError"];
	    }
	}
	export class LogoutResult {
	    remoteInvalidated: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LogoutResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.remoteInvalidated = source["remoteInvalidated"];
	        this.message = source["message"];
	    }
	}
	export class Profile {
	    userId: string;
	    name: string;
	    shortName: string;
	    email: string;
	    broker: string;
	    avatarUrl: string;
	    initials: string;
	    exchanges: string[];
	    products: string[];
	    orderTypes: string[];
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.userId = source["userId"];
	        this.name = source["name"];
	        this.shortName = source["shortName"];
	        this.email = source["email"];
	        this.broker = source["broker"];
	        this.avatarUrl = source["avatarUrl"];
	        this.initials = source["initials"];
	        this.exchanges = source["exchanges"];
	        this.products = source["products"];
	        this.orderTypes = source["orderTypes"];
	    }
	}

}

export namespace orders {
	
	export class SectionCounts {
	    open: number;
	    executed: number;
	    rejected: number;
	    cancelled: number;
	
	    static createFrom(source: any = {}) {
	        return new SectionCounts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.open = source["open"];
	        this.executed = source["executed"];
	        this.rejected = source["rejected"];
	        this.cancelled = source["cancelled"];
	    }
	}
	export class OrderRow {
	    orderId: string;
	    timeMs: number;
	    side: string;
	    exchange: string;
	    symbol: string;
	    token: number;
	    product: string;
	    orderType: string;
	    variety: string;
	    qty: number;
	    filledQty: number;
	    pricePaise: number;
	    triggerPaise: number;
	    avgPricePaise: number;
	    status: string;
	    statusMessage: string;
	    section: string;
	    tag: string;
	    source: string;
	    outOfScope: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OrderRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.orderId = source["orderId"];
	        this.timeMs = source["timeMs"];
	        this.side = source["side"];
	        this.exchange = source["exchange"];
	        this.symbol = source["symbol"];
	        this.token = source["token"];
	        this.product = source["product"];
	        this.orderType = source["orderType"];
	        this.variety = source["variety"];
	        this.qty = source["qty"];
	        this.filledQty = source["filledQty"];
	        this.pricePaise = source["pricePaise"];
	        this.triggerPaise = source["triggerPaise"];
	        this.avgPricePaise = source["avgPricePaise"];
	        this.status = source["status"];
	        this.statusMessage = source["statusMessage"];
	        this.section = source["section"];
	        this.tag = source["tag"];
	        this.source = source["source"];
	        this.outOfScope = source["outOfScope"];
	    }
	}
	export class BookSnapshot {
	    orders: OrderRow[];
	    counts: SectionCounts;
	    outOfScopeCount: number;
	    asOfMs: number;
	    stale: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BookSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.orders = this.convertValues(source["orders"], OrderRow);
	        this.counts = this.convertValues(source["counts"], SectionCounts);
	        this.outOfScopeCount = source["outOfScopeCount"];
	        this.asOfMs = source["asOfMs"];
	        this.stale = source["stale"];
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
	export class HoldingRow {
	    exchange: string;
	    symbol: string;
	    token: number;
	    qty: number;
	    avgPaise: number;
	    ltpPaise: number;
	    prevClosePaise: number;
	    investedPaise: number;
	    currentPaise: number;
	    pnlPaise: number;
	    netChangeBp: number;
	    dayChangePaise: number;
	    dayChangeBp: number;
	    outOfScope: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HoldingRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exchange = source["exchange"];
	        this.symbol = source["symbol"];
	        this.token = source["token"];
	        this.qty = source["qty"];
	        this.avgPaise = source["avgPaise"];
	        this.ltpPaise = source["ltpPaise"];
	        this.prevClosePaise = source["prevClosePaise"];
	        this.investedPaise = source["investedPaise"];
	        this.currentPaise = source["currentPaise"];
	        this.pnlPaise = source["pnlPaise"];
	        this.netChangeBp = source["netChangeBp"];
	        this.dayChangePaise = source["dayChangePaise"];
	        this.dayChangeBp = source["dayChangeBp"];
	        this.outOfScope = source["outOfScope"];
	    }
	}
	export class HoldingsSummary {
	    empty: boolean;
	    currentValuePaise: number;
	    investmentPaise: number;
	    pnlPaise: number;
	    pnlBp: number;
	    dayChangePaise: number;
	    outOfScopeCount: number;
	
	    static createFrom(source: any = {}) {
	        return new HoldingsSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.empty = source["empty"];
	        this.currentValuePaise = source["currentValuePaise"];
	        this.investmentPaise = source["investmentPaise"];
	        this.pnlPaise = source["pnlPaise"];
	        this.pnlBp = source["pnlBp"];
	        this.dayChangePaise = source["dayChangePaise"];
	        this.outOfScopeCount = source["outOfScopeCount"];
	    }
	}
	export class HoldingsSnapshot {
	    rows: HoldingRow[];
	    summary: HoldingsSummary;
	    asOfMs: number;
	    stale: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HoldingsSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rows = this.convertValues(source["rows"], HoldingRow);
	        this.summary = this.convertValues(source["summary"], HoldingsSummary);
	        this.asOfMs = source["asOfMs"];
	        this.stale = source["stale"];
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
	
	
	export class PositionRow {
	    key: string;
	    exchange: string;
	    symbol: string;
	    token: number;
	    product: string;
	    qty: number;
	    multiplier: number;
	    avgPaise: number;
	    ltpPaise: number;
	    pnlPaise: number;
	    netChangeBp: number;
	    closed: boolean;
	    outOfScope: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PositionRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.exchange = source["exchange"];
	        this.symbol = source["symbol"];
	        this.token = source["token"];
	        this.product = source["product"];
	        this.qty = source["qty"];
	        this.multiplier = source["multiplier"];
	        this.avgPaise = source["avgPaise"];
	        this.ltpPaise = source["ltpPaise"];
	        this.pnlPaise = source["pnlPaise"];
	        this.netChangeBp = source["netChangeBp"];
	        this.closed = source["closed"];
	        this.outOfScope = source["outOfScope"];
	    }
	}
	export class PositionsSnapshot {
	    net: PositionRow[];
	    day: PositionRow[];
	    netTotalPnlPaise: number;
	    dayTotalPnlPaise: number;
	    outOfScopeCount: number;
	    asOfMs: number;
	    stale: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PositionsSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.net = this.convertValues(source["net"], PositionRow);
	        this.day = this.convertValues(source["day"], PositionRow);
	        this.netTotalPnlPaise = source["netTotalPnlPaise"];
	        this.dayTotalPnlPaise = source["dayTotalPnlPaise"];
	        this.outOfScopeCount = source["outOfScopeCount"];
	        this.asOfMs = source["asOfMs"];
	        this.stale = source["stale"];
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

export namespace service {
	
	export class Licence {
	    name: string;
	    version: string;
	    licence: string;
	
	    static createFrom(source: any = {}) {
	        return new Licence(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.licence = source["licence"];
	    }
	}
	export class AppInfo {
	    version: string;
	    build: string;
	    schemaVersion: number;
	    dataDir: string;
	    licences: Licence[];
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.build = source["build"];
	        this.schemaVersion = source["schemaVersion"];
	        this.dataDir = source["dataDir"];
	        this.licences = this.convertValues(source["licences"], Licence);
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
	export class BackupInfo {
	    path: string;
	    ok: boolean;
	    reason: string;
	    schema: number;
	    supportedSchema: number;
	    appVersion: string;
	    createdAt: string;
	    counts: Record<string, number>;
	    includesCandles: boolean;
	    sizeBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new BackupInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.ok = source["ok"];
	        this.reason = source["reason"];
	        this.schema = source["schema"];
	        this.supportedSchema = source["supportedSchema"];
	        this.appVersion = source["appVersion"];
	        this.createdAt = source["createdAt"];
	        this.counts = source["counts"];
	        this.includesCandles = source["includesCandles"];
	        this.sizeBytes = source["sizeBytes"];
	    }
	}
	export class PhaseDTO {
	    kind: string;
	    start: string;
	    end: string;
	
	    static createFrom(source: any = {}) {
	        return new PhaseDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.start = source["start"];
	        this.end = source["end"];
	    }
	}
	export class CalendarEntryDTO {
	    exchange: string;
	    date: string;
	    type: string;
	    phases: PhaseDTO[];
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new CalendarEntryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exchange = source["exchange"];
	        this.date = source["date"];
	        this.type = source["type"];
	        this.phases = this.convertValues(source["phases"], PhaseDTO);
	        this.source = source["source"];
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
	export class CalendarEntryInput {
	    exchange: string;
	    date: string;
	    type: string;
	    phases: PhaseDTO[];
	
	    static createFrom(source: any = {}) {
	        return new CalendarEntryInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exchange = source["exchange"];
	        this.date = source["date"];
	        this.type = source["type"];
	        this.phases = this.convertValues(source["phases"], PhaseDTO);
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
	export class CalendarInfo {
	    version: string;
	    coversUntil: string;
	    stale: boolean;
	    entries: CalendarEntryDTO[];
	
	    static createFrom(source: any = {}) {
	        return new CalendarInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.coversUntil = source["coversUntil"];
	        this.stale = source["stale"];
	        this.entries = this.convertValues(source["entries"], CalendarEntryDTO);
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
	export class CandleDTO {
	    t: number;
	    o: number;
	    h: number;
	    l: number;
	    c: number;
	    v: number;
	    oi: number;
	    complete: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CandleDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.t = source["t"];
	        this.o = source["o"];
	        this.h = source["h"];
	        this.l = source["l"];
	        this.c = source["c"];
	        this.v = source["v"];
	        this.oi = source["oi"];
	        this.complete = source["complete"];
	    }
	}
	export class CandlesRequest {
	    token: number;
	    interval: string;
	    fromMs: number;
	    toMs: number;
	    tradingDays: number;
	
	    static createFrom(source: any = {}) {
	        return new CandlesRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.interval = source["interval"];
	        this.fromMs = source["fromMs"];
	        this.toMs = source["toMs"];
	        this.tradingDays = source["tradingDays"];
	    }
	}
	export class CandlesResult {
	    candles: CandleDTO[];
	    complete: boolean;
	    backfillJobId: string;
	    needsDataPlan: boolean;
	    offline: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CandlesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.candles = this.convertValues(source["candles"], CandleDTO);
	        this.complete = source["complete"];
	        this.backfillJobId = source["backfillJobId"];
	        this.needsDataPlan = source["needsDataPlan"];
	        this.offline = source["offline"];
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
	export class CrashInfo {
	    path: string;
	    atMs: number;
	
	    static createFrom(source: any = {}) {
	        return new CrashInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.atMs = source["atMs"];
	    }
	}
	export class CredentialsInput {
	    apiKey: string;
	    apiSecret: string;
	
	    static createFrom(source: any = {}) {
	        return new CredentialsInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiKey = source["apiKey"];
	        this.apiSecret = source["apiSecret"];
	    }
	}
	export class DataStats {
	    dataDir: string;
	    appDbBytes: number;
	    candleBytes: number;
	    candleBudgetBytes: number;
	    instrumentsImportedAtMs: number;
	    backfillOpenJobs: number;
	
	    static createFrom(source: any = {}) {
	        return new DataStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dataDir = source["dataDir"];
	        this.appDbBytes = source["appDbBytes"];
	        this.candleBytes = source["candleBytes"];
	        this.candleBudgetBytes = source["candleBudgetBytes"];
	        this.instrumentsImportedAtMs = source["instrumentsImportedAtMs"];
	        this.backfillOpenJobs = source["backfillOpenJobs"];
	    }
	}
	export class FeedStateDTO {
	    state: string;
	    attempt: number;
	    nextRetryMs: number;
	    lastTickMs: number;
	    connections: number;
	    tokens: number;
	
	    static createFrom(source: any = {}) {
	        return new FeedStateDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.attempt = source["attempt"];
	        this.nextRetryMs = source["nextRetryMs"];
	        this.lastTickMs = source["lastTickMs"];
	        this.connections = source["connections"];
	        this.tokens = source["tokens"];
	    }
	}
	export class InstrumentDTO {
	    token: number;
	    exchange: string;
	    segment: string;
	    tag: string;
	    tradingsymbol: string;
	    name: string;
	    displayName: string;
	    instrumentType: string;
	    expiryMs: number;
	    tickSizePaise: number;
	    lotSize: number;
	    isIndex: boolean;
	    isFuture: boolean;
	    underlying: string;
	    alsoOnNse: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InstrumentDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.exchange = source["exchange"];
	        this.segment = source["segment"];
	        this.tag = source["tag"];
	        this.tradingsymbol = source["tradingsymbol"];
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.instrumentType = source["instrumentType"];
	        this.expiryMs = source["expiryMs"];
	        this.tickSizePaise = source["tickSizePaise"];
	        this.lotSize = source["lotSize"];
	        this.isIndex = source["isIndex"];
	        this.isFuture = source["isFuture"];
	        this.underlying = source["underlying"];
	        this.alsoOnNse = source["alsoOnNse"];
	    }
	}
	
	export class MarketStatus {
	    exchange: string;
	    phase: string;
	    dayType: string;
	    label: string;
	    tooltip: string;
	    atMs: number;
	
	    static createFrom(source: any = {}) {
	        return new MarketStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exchange = source["exchange"];
	        this.phase = source["phase"];
	        this.dayType = source["dayType"];
	        this.label = source["label"];
	        this.tooltip = source["tooltip"];
	        this.atMs = source["atMs"];
	    }
	}
	export class OffsetDTO {
	    millis: number;
	    source: string;
	    samples: number;
	    warn: boolean;
	    skewed: boolean;
	    updatedAtMs: number;
	
	    static createFrom(source: any = {}) {
	        return new OffsetDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.millis = source["millis"];
	        this.source = source["source"];
	        this.samples = source["samples"];
	        this.warn = source["warn"];
	        this.skewed = source["skewed"];
	        this.updatedAtMs = source["updatedAtMs"];
	    }
	}
	
	export class RefreshRequest {
	    token: number;
	    interval: string;
	    fromMs: number;
	    toMs: number;
	
	    static createFrom(source: any = {}) {
	        return new RefreshRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.interval = source["interval"];
	        this.fromMs = source["fromMs"];
	        this.toMs = source["toMs"];
	    }
	}
	export class RemovedItem {
	    listId: string;
	    token: number;
	    position: number;
	
	    static createFrom(source: any = {}) {
	        return new RemovedItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.listId = source["listId"];
	        this.token = source["token"];
	        this.position = source["position"];
	    }
	}
	export class SecretStorageInfo {
	    mode: string;
	    backend: string;
	    keyringAvailable: boolean;
	    fileExists: boolean;
	    choices: string[];
	
	    static createFrom(source: any = {}) {
	        return new SecretStorageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.backend = source["backend"];
	        this.keyringAvailable = source["keyringAvailable"];
	        this.fileExists = source["fileExists"];
	        this.choices = source["choices"];
	    }
	}
	export class SessionStatus {
	    state: string;
	    sessionId: string;
	    userId: string;
	    loggedInAtMs: number;
	    expiresAtMs: number;
	    dataPlan: string;
	    offline: boolean;
	    riskAckRequired: boolean;
	    errorCode: string;
	    errorMessage: string;
	    credentials: kite.CredentialsInfo;
	    secrets: SecretStorageInfo;
	
	    static createFrom(source: any = {}) {
	        return new SessionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.sessionId = source["sessionId"];
	        this.userId = source["userId"];
	        this.loggedInAtMs = source["loggedInAtMs"];
	        this.expiresAtMs = source["expiresAtMs"];
	        this.dataPlan = source["dataPlan"];
	        this.offline = source["offline"];
	        this.riskAckRequired = source["riskAckRequired"];
	        this.errorCode = source["errorCode"];
	        this.errorMessage = source["errorMessage"];
	        this.credentials = this.convertValues(source["credentials"], kite.CredentialsInfo);
	        this.secrets = this.convertValues(source["secrets"], SecretStorageInfo);
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
	export class SettingValue {
	    name: string;
	    valueJson: string;
	
	    static createFrom(source: any = {}) {
	        return new SettingValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.valueJson = source["valueJson"];
	    }
	}
	export class StatusSnapshot {
	    feed: FeedStateDTO;
	    market: MarketStatus;
	    clock: OffsetDTO;
	
	    static createFrom(source: any = {}) {
	        return new StatusSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.feed = this.convertValues(source["feed"], FeedStateDTO);
	        this.market = this.convertValues(source["market"], MarketStatus);
	        this.clock = this.convertValues(source["clock"], OffsetDTO);
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
	export class TickDTO {
	    token: number;
	    ltp: number;
	    close: number;
	    change: number;
	    changePct: number;
	    volume: number;
	    tsMs: number;
	    polled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TickDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.ltp = source["ltp"];
	        this.close = source["close"];
	        this.change = source["change"];
	        this.changePct = source["changePct"];
	        this.volume = source["volume"];
	        this.tsMs = source["tsMs"];
	        this.polled = source["polled"];
	    }
	}
	export class WatchResult {
	    live: number[];
	    polled: number[];
	    snapshot: TickDTO[];
	
	    static createFrom(source: any = {}) {
	        return new WatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.live = source["live"];
	        this.polled = source["polled"];
	        this.snapshot = this.convertValues(source["snapshot"], TickDTO);
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
	export class WatchlistItem {
	    token: number;
	    exchange: string;
	    symbol: string;
	    displayName: string;
	    name: string;
	    isIndex: boolean;
	    missing: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WatchlistItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.exchange = source["exchange"];
	        this.symbol = source["symbol"];
	        this.displayName = source["displayName"];
	        this.name = source["name"];
	        this.isIndex = source["isIndex"];
	        this.missing = source["missing"];
	    }
	}
	export class Watchlist {
	    id: string;
	    name: string;
	    items: WatchlistItem[];
	
	    static createFrom(source: any = {}) {
	        return new Watchlist(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.items = this.convertValues(source["items"], WatchlistItem);
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
	
	export class WatchlistsState {
	    lists: Watchlist[];
	    activeId: string;
	
	    static createFrom(source: any = {}) {
	        return new WatchlistsState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lists = this.convertValues(source["lists"], Watchlist);
	        this.activeId = source["activeId"];
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

export namespace settings {
	
	export class ShortcutAction {
	    id: string;
	    label: string;
	    scope: string;
	    defaultBinding: string;
	    binding: string;
	
	    static createFrom(source: any = {}) {
	        return new ShortcutAction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.scope = source["scope"];
	        this.defaultBinding = source["defaultBinding"];
	        this.binding = source["binding"];
	    }
	}

}

