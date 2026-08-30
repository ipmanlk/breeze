import { css } from "lit";

/**
 * Shared animation CSS for Lit components.
 *
 * These rules duplicate the class selectors AND the matching keyframes
 * from animations.css so that shadow DOM components can use them.
 * CSS selectors do not pierce shadow roots, and some shadow-root
 * stylesheets cannot resolve keyframes defined in the global document.
 *
 * Import into a component's static styles array:
 *
 *   static styles = [
 *     pageEnterStyles,
 *     css`
 *       :host { ... }
 *     `,
 *   ];
 */

export const pageEnterStyles = css`
  @keyframes page-enter {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  .page-enter {
    animation: page-enter var(--dur-entrance) var(--ease-2);
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    min-width: 0;
    /* NOTE: no transform in the keyframes and no will-change: transform.
      A non-none transform (or will-change: transform) on this wrapper
      would establish a containing block for position: fixed descendants,
      breaking every fixed dropdown panel (select/combobox/popover/date-field)
      rendered inside the page: they would position relative to this wrapper
      instead of the viewport. An opacity-only entrance avoids that. */
  }
`;

export const contentEnterStyles = css`
  @keyframes content-in {
    from {
      opacity: 0;
      transform: translateY(4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .content-enter {
    animation: content-in var(--dur-normal) var(--ease-2);
  }
`;

export const listItemEnterStyles = css`
  @keyframes list-item-in {
    from {
      opacity: 0;
      transform: translateY(-8px) scale(0.98);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }

  .list-item-enter {
    animation: list-item-in var(--dur-normal) var(--ease-2);
  }
`;

export const tabContentStyles = css`
  @keyframes tab-fade {
    from {
      opacity: 0.6;
    }
    to {
      opacity: 1;
    }
  }

  .tab-content {
    animation: tab-fade var(--dur-normal) var(--ease-1);
  }
`;

export const slideInRightStyles = css`
  @keyframes slide-in-right {
    from {
      transform: translateX(100%);
    }
    to {
      transform: translateX(0);
    }
  }

  .slide-in-right {
    animation: slide-in-right var(--dur-slow) var(--ease-2);
  }
`;

export const slideOutRightStyles = css`
  @keyframes slide-out-right {
    from {
      transform: translateX(0);
    }
    to {
      transform: translateX(100%);
    }
  }

  .slide-out-right {
    animation: slide-out-right var(--dur-exit) var(--ease-3);
  }
`;

export const emptyEnterStyles = css`
  @keyframes empty-in {
    from {
      opacity: 0;
      transform: translateY(12px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .empty-enter {
    animation: empty-in var(--dur-slow) var(--ease-2);
  }
`;
