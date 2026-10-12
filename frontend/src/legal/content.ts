// Our own text (FR-ACCT-015, FR-SHELL-020). No sentence is taken from any other product.
export interface LegalPage { title: string; updatedFor: string; paragraphs: string[] }

export const riskDisclosure: LegalPage = {
  title: 'Risk disclosure',
  updatedFor: '0.1.0',
  paragraphs: [
    'Tradeloom is an educational and decision-support tool. It helps you study markets, test ideas and follow your Kite account; it does not give investment advice.',
    'Backtest results and paper trades are hypothetical. They are computed on past or simulated prices and cannot include every real cost, delay or rejection. Past results do not predict future returns.',
    'You are responsible for every order sent from your Kite account, including orders you confirm in Tradeloom. Trading equities and futures can lose more money than you expect.',
    'Tradeloom never sends an order on its own unless you explicitly arm automatic execution for a deployment, and arming ends with each Kite session.',
  ],
};
