import { watchQuotes } from '../api/quotes';
import { applySnapshot } from './quotes';
import { SubscriptionCoordinator } from './subscriptions';

/**
 * The app's single coordinator: one merged Quotes.Watch per consumer (ARCHITECTURE §8.2).
 * Removals are sent at once: Go's Quotes.Watch already keeps removed tokens for 30 s (UnsubscribeLinger,
 * FR-WATCH-006) and keeps a token re-added within that window, so a second linger here would double it.
 */
export const subscriptions = new SubscriptionCoordinator({ watch: watchQuotes, onSnapshot: applySnapshot, releaseDelayMs: 0 });
