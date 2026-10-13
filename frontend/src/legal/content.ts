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

export const termsOfUse: LegalPage = {
  title: 'Terms of use',
  updatedFor: '0.1.0',
  paragraphs: [
    'Tradeloom is a desktop application for studying the Indian equity and futures markets, testing trading ideas on historical data and following your own Zerodha Kite account. It is an educational and decision-support tool.',
    '## Your responsibility',
    'You decide every trade. Any order sent from your Kite account, including one you confirm inside Tradeloom, is your own instruction to your broker, and you bear its outcome.',
    '## No automatic trading unless you arm it',
    'Tradeloom does not send orders by itself. Automatic execution exists only where you explicitly arm it for a deployment, within limits you set, and arming ends when your Kite session ends.',
    '## No warranty',
    'Tradeloom is provided as is. Prices, indicators, backtests and alerts can be late, incomplete or wrong because of data, network or software faults. Check important information in Kite before acting on it.',
    '## Your Kite account',
    'You need your own Kite Connect app and must follow the terms of your broker and the exchanges, including the rules for trading through an API.',
    '## Words used in the app',
    [
      '- Equity Builder: where you write a strategy.',
      '- Scanner builder: where you write a scanner.',
      '- Execute: the builder action that starts a backtest or a deployment.',
      '- Backtest: a simulation of a strategy on past candles.',
      '- Deployment: a strategy running live in one execution mode — Notify & confirm, Signal-only, Paper or Armed auto.',
      '- Order ticket: the single dialog in which every order is reviewed before it is sent.',
      '- Universe: a named list of instruments for scanners.',
      '- Library: the bundled collection of ready-made strategies and scanners.',
      '- Kill switch: "Stop all trading" — cancels pending orders and disarms every deployment.',
      '- Status indicator: the top-bar signal for market data, Kite session and market phase.',
    ].join('\n'),
  ],
};

export const privacyNotice: LegalPage = {
  title: 'Privacy notice',
  updatedFor: '0.1.0',
  paragraphs: [
    'Tradeloom runs entirely on your computer. Your strategies, scanners, watchlists, settings, backtests and candle data are stored in your data folder and are not sent anywhere.',
    '## Network connections',
    'Tradeloom connects only to:',
    [
      '- Zerodha Kite (api.kite.trade and ws.kite.trade) for your account, prices and history; you log in on the Kite website in your own browser.',
      '- The Telegram Bot API, only if you turn on the Telegram bot.',
      '- The update feed you configure, only if you turn on update checks.',
      '- The IP-check address you configure, only if you turn on the static-IP check.',
    ].join('\n'),
    '## No telemetry',
    'Tradeloom collects no usage statistics, sends no crash reports and contains no analytics.',
    '## Secrets',
    'Your Kite API secret, daily access token and any Telegram bot token are kept in your system keyring or in an encrypted file protected by your passphrase. They never appear in logs, backups or support bundles.',
    '## Support bundles',
    'A support bundle is created only when you ask for one. It contains log files, settings without secrets, schema, library and calendar versions, the last 200 order-journal rows and recent deployment log excerpts. You decide whether to share it.',
  ],
};

export const LEGAL_PAGES: Record<string, LegalPage> = { terms: termsOfUse, privacy: privacyNotice, risk: riskDisclosure };
