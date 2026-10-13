/** Elements that are ARIA live regions (explicit aria-live other than "off", or a role with an implicit live value). */
const LIVE = '[aria-live]:not([aria-live="off"]), [role="status"], [role="alert"], [role="log"], [role="marquee"], [role="timer"]';

/** Live regions nested inside another live region (Ruling 86 M5): screen readers may announce them twice. */
export function nestedLiveRegions(root: ParentNode = document): Element[] {
  return [...root.querySelectorAll(LIVE)].filter((el) => el.parentElement?.closest(LIVE) != null);
}
