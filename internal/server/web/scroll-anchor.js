// scroll-anchor.js — "anchor to bottom when at bottom, show pill otherwise"
//
// Usage:
//   const anchor = attachScrollAnchor(scrollEl, pillEl);
//   // call anchor.onContentChange() after appending messages
//   // anchor.stick() forces scroll to bottom and hides pill

const NEAR_BOTTOM_PX = 48;

export function attachScrollAnchor(scrollEl, pillEl, { onJump } = {}) {
  let atBottom = true;
  let unread = 0;

  function isAtBottom() {
    const gap = scrollEl.scrollHeight - scrollEl.scrollTop - scrollEl.clientHeight;
    return gap <= NEAR_BOTTOM_PX;
  }

  function updatePill() {
    if (!pillEl) return;
    if (atBottom || unread === 0) {
      pillEl.hidden = true;
      unread = 0;
    } else {
      pillEl.hidden = false;
      const countEl = pillEl.querySelector('[data-unread-count]') || pillEl;
      countEl.textContent = unread + ' new ↓';
    }
  }

  function stick() {
    scrollEl.scrollTop = scrollEl.scrollHeight;
    atBottom = true;
    unread = 0;
    updatePill();
  }

  scrollEl.addEventListener('scroll', () => {
    const wasAtBottom = atBottom;
    atBottom = isAtBottom();
    if (!wasAtBottom && atBottom) {
      unread = 0;
      updatePill();
    }
  });

  if (pillEl) {
    pillEl.addEventListener('click', () => {
      stick();
      onJump?.();
    });
  }

  return {
    stick,
    onContentChange({ prepended = false, count = 1 } = {}) {
      if (prepended) {
        // content added above — doesn't affect bottom-tracking except for pill semantics
        return;
      }
      if (atBottom) {
        // pin to bottom after DOM update
        requestAnimationFrame(() => {
          scrollEl.scrollTop = scrollEl.scrollHeight;
        });
      } else {
        unread += count;
        updatePill();
      }
    },
    isAtBottom: () => atBottom,
    forceAtBottomCheck() {
      atBottom = isAtBottom();
      updatePill();
    },
    resetUnread() {
      unread = 0;
      updatePill();
    },
  };
}
