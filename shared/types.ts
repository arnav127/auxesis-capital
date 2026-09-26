// Shapes of the /api/aux/* responses (see backend/fund/views.go).

export interface GrowthPoint { d: string; f: number; b?: number }

export interface PublicView {
  hasData: boolean;
  asOf: string;
  inception: string;
  fundItd: number;
  nifty50Itd: number | null;
  nifty500Itd: number | null;
  positions: number;
  pods: number;
  growth: GrowthPoint[];
  motto: string;
}

export interface MeView {
  user: { name: string; email: string; role: 'investor' | 'admin' };
  investor: { name: string; folio: string } | null;
}

export interface Risk {
  days: number;
  fundReturn: number;
  benchReturn: number;
  bench500Return: number;
  cagr: number;
  benchCagr: number;
  annualised: boolean;
  alpha: number;
  beta: number;
  volatility: number;
  sharpe: number;
  sortino: number;
  information: number;
  trackingError: number;
  maxDrawdown: number;
  upCapture: number;
  downCapture: number;
  riskFree: number;
  hasBenchmark: boolean;
}

export interface Position {
  symbol: string;
  name: string;
  sector: string;
  pods: string[] | null;
  qty: number;
  avgCost: number;
  price: number;
  prevClose: number;
  priceDate: string;
  stale: boolean;
  value: number;
  weight: number;
  day1: number;
  return: number;
  unrealised: number;
  since: string;
}

export interface PodStat { pod: string; trades: number; open: number; invested: number; value: number; realised: number; unrealised: number; pnl: number }
export interface Sector { name: string; value: number; weight: number }
export interface MonthRow { year: number; months: (number | null)[]; ytd: number }
export interface Txn { date: string; kind: string; amount: number; units: number; nav: number; note: string }

export interface Holding {
  id: string;
  name: string;
  email: string;
  folio: string;
  programme: string;
  units: number;
  invested: number;
  value: number;
  return: number;
  xirr: number | null;
  since: string;
  txns: Txn[] | null;
}

export interface FundSettings { startNav: number; riskFree: number; managerNote: string; managerNoteBy: string; managerNoteDate: string; motto: string }

export interface PortfolioView {
  asOf: string;
  inception: string;
  lastSync: string | null;
  nav: number;
  navChange: number;
  startNav: number;
  aum: number;
  units: number;
  series: { d: string; nav: number; n50?: number }[];
  risk: Risk;
  positions: Position[];
  cash: number;
  cashWeight: number;
  sectors: Sector[];
  pods: PodStat[];
  monthly: MonthRow[];
  realised: number;
  settings: FundSettings;
  me: Holding | null;
  warnings?: string[];
}

export interface ReportMeta {
  slug: string;
  title: string;
  type: string;
  category: 'Letters' | 'Factsheets' | 'Research' | 'Macro';
  date: string;
  access: 'public' | 'investors';
  author: string;
  dek: string;
  summary: string;
  kicker: string;
  kickerSub: string;
  pages: string;
  readMins: number;
  hasPdf: boolean;
  locked: boolean;
}

export interface Fact { k: string; v: string; up?: boolean }

export interface Report extends ReportMeta {
  facts: Fact[] | null;
  body: string;
  preview: boolean;
  toc: string[];
  next: ReportMeta | null;
}

export interface Member { id: string; name: string; role: string; group: 'leadership' | 'pgp2' | 'pgp1'; org: string; batch: string; photo: string; linkedin: string }

export interface SyncRow { at: string; source: string; ok: boolean; message: string; trades: number; warnings: string[] | null; by: string }

export interface AdminStatus {
  excelUrl: boolean;
  priceHistory: boolean;
  syncs: SyncRow[];
  counts: Record<string, number>;
  flowUnits: number;
  flowAmount: number;
  investorUnits: number;
  investors: Holding[];
  warnings: string[] | null;
  noHistory: string[];
  asOf: string;
  nav: number;
}

export interface SyncSummary { ok: boolean; message: string; trades: number; warnings: string[] | null }
