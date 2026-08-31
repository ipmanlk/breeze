import { logError } from "@/lib/log";
import { generateKeyBetween } from "@/lib/lexorank";
import { createRef, ref } from "lit/directives/ref.js";
import { computeGap, computeGapY } from "@/lib/dnd-gap";
import { css, html, LitElement, nothing } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
import { contentEnterStyles } from "@/styles/shared-animations";
import type {
  DtoAttachmentResponse,
  DtoCycleResponse,
  DtoProjectMemberResponse,
  DtoProjectResponse,
  DtoTaskActivityResponse,
  DtoTaskResponse,
  DtoTaskStatusResponse,
  DtoTimeEntryResponse,
} from "@/api";
import {
  deleteProjectsByIdTasksByTaskIdAttachmentsByAttachmentId,
  deleteProjectsByIdTasksByTaskIdTimeEntriesByEntryId,
  getProjectsByIdCycles,
  getProjectsByIdMembers,
  getProjectsByIdTasksByTaskIdActivity,
  getProjectsByIdTasksByTaskIdAttachments,
  getProjectsByIdTasksByTaskIdSubtasks,
  getProjectsByIdTasksByTaskIdTimeEntries,
  postProjectsByIdTasksByTaskIdAttachments,
  postProjectsByIdTasksByTaskIdTimeEntries,
  postProjectsByIdTasksByTaskIdTimeEntriesStart,
  postProjectsByIdTasksByTaskIdTimeEntriesStop,
} from "@/api";
import {
  addTaskDependency,
  createTask,
  deleteTask,
  duplicateTask,
  fetchTaskDependencies,
  moveTaskToProject,
  projectDetail,
  removeTaskDependency,
  reorderSubtasks,
  selectTask,
  setTaskLabels,
  updateTask,
} from "@/store/project-detail";
import { projects as projectsSignal } from "@/store/projects";
import { showToast } from "@/components/ui/toast-store";
import { SignalController } from "@/lib/signal-controller";
import { OutsideClickController } from "@/lib/outside-click-controller";
import { timeAgo, timeAgoShort } from "@/lib/format/time-ago";
import "../../components/ui/dialog.ts";
import "../../components/ui/select.ts";
import "../../components/ui/combobox.ts";
import "../../components/ui/popover.ts";
import "../../components/ui/date-field.ts";
import "../../components/ui/tabs.ts";
import "../../components/ui/input.ts";
import "../../components/ui/field.ts";
import "../../components/ui/button.ts";
import "../../components/ui/label-picker.ts";
import "../../components/ui/plume-icon.ts";
import "../../components/ui/avatar.ts";
import "@/features/comments/comment-thread.ts";
import { PlumeTaskEditor } from "@/components/plume-task-editor.ts";
import { buildResolver, resolveLabel } from "@/features/chat/mention-utils";
import { localized, msg } from "@lit/localize";

function getPriorities() {
  return [
    {
      value: "none",
      label: msg("No priority"),
      color: "var(--muted-foreground)",
    },
    { value: "low", label: msg("Low"), color: "oklch(0.7 0.15 250)" },
    { value: "medium", label: msg("Medium"), color: "oklch(0.8 0.15 85)" },
    { value: "high", label: msg("High"), color: "oklch(0.7 0.18 50)" },
    { value: "urgent", label: msg("Urgent"), color: "oklch(0.6 0.22 27)" },
  ];
}

/**
 * Plume task detail dialog: full task view/edit modal.
 *
 * Properties (all `attribute: false`):
 *  - `task`    : the task to display
 *  - `project` : parent project (for cycles, project ID)
 *  - `statuses`: available task statuses
 *
 * Events:
 *  - `close`  : dispatched when dialog closes
 *  - `delete` : dispatched when task is deleted (detail = taskId)
 */
@localized()
@customElement("plume-task-detail-dialog")
export class PlumeTaskDetailDialog extends LitElement {
  static styles = [
    contentEnterStyles,
    css`
      *,
      *::before,
      *::after {
        box-sizing: border-box;
      }
      :host {
        display: contents;
      }

      /* Layout */
      .tdd-layout {
        display: flex;
        flex-direction: column;
        height: 100%;
        overflow: hidden;
      }
      .tdd-header {
        flex-shrink: 0;
        padding: var(--space-5) var(--space-6) var(--space-3);
        border-bottom: 1px solid var(--border);
      }
      .tdd-title-row {
        display: flex;
        align-items: flex-start;
        gap: var(--space-2);
      }
      .tdd-title {
        flex: 1;
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
        line-height: 1.3;
        cursor: pointer;
        color: var(--foreground);
      }
      .tdd-title:hover {
        color: color-mix(in oklch, var(--foreground) 80%, transparent);
      }
      .tdd-meta {
        margin-top: var(--space-1);
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      .tdd-actions {
        position: relative;
        display: flex;
        align-items: center;
        gap: var(--space-0-5);
        flex-shrink: 0;
        margin-top: calc(var(--space-0-5) * -1);
      }
      .tdd-actions-btn {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: var(--space-7);
        height: var(--space-7);
        border: none;
        border-radius: var(--radius-md);
        background: transparent;
        color: var(--muted-foreground);
        cursor: pointer;
        transition:
          background var(--dur-fast) var(--ease-1),
          color var(--dur-fast) var(--ease-1);
      }
      .tdd-actions-btn:hover {
        background: var(--accent);
        color: var(--foreground);
      }

      /* Actions dropdown */
      .tdd-dropdown {
        position: absolute;
        top: calc(100% + var(--space-1));
        right: 0;
        z-index: var(--z-dropdown);
        width: var(--space-44);
        border: 1px solid var(--border);
        border-radius: var(--radius-md);
        background: var(--popover);
        color: var(--popover-foreground);
        box-shadow: var(--shadow-md);
        padding: var(--space-1);
      }
      .tdd-dropdown-item {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        width: 100%;
        padding: var(--space-1-5) var(--space-2);
        border: none;
        border-radius: var(--radius-sm);
        background: transparent;
        color: inherit;
        font-size: var(--text-sm);
        font-family: inherit;
        text-align: left;
        cursor: pointer;
        transition: background var(--dur-fast) var(--ease-1);
      }
      .tdd-dropdown-item:hover {
        background: var(--accent);
      }
      .tdd-dropdown-item.danger {
        color: var(--destructive);
      }
      .tdd-dropdown-item.danger:hover {
        background: var(--destructive);
        color: var(--destructive-foreground);
      }
      .tdd-dropdown-divider {
        height: 1px;
        background: var(--border);
        margin: var(--space-1) var(--space-2);
      }
      .tdd-shortcut {
        margin-left: auto;
        font-size: var(--text-2xs);
        color: var(--muted-foreground);
      }

      /* Content area */
      .tdd-content {
        flex: 1;
        display: flex;
        min-height: 0;
        overflow: hidden;
      }
      .tdd-main {
        flex: 1;
        min-width: 0;
        display: flex;
        flex-direction: column;
        overflow: hidden;
      }
      .tdd-tabs-wrap {
        flex-shrink: 0;
        padding: var(--space-2) var(--space-6) 0;
        border-bottom: 1px solid var(--border);
      }
      .tdd-tab-body {
        flex: 1;
        min-height: 0;
        overflow-y: auto;
        padding: var(--space-4) var(--space-6);
      }

      /* Sidebar */
      .tdd-sidebar {
        width: var(--space-80);
        flex-shrink: 0;
        border-left: 1px solid var(--border);
        overflow-y: auto;
        padding: var(--space-5);
      }
      .tdd-sidebar-title {
        font-size: var(--text-sm);
        font-weight: 500;
        color: var(--muted-foreground);
        border-bottom: 1px solid var(--border);
        padding-bottom: var(--space-2);
        margin-bottom: var(--space-3);
      }
      .tdd-prop {
        display: flex;
        align-items: center;
        gap: var(--space-3);
      }
      .tdd-prop + .tdd-prop {
        margin-top: var(--space-2-5);
      }
      .tdd-prop-label {
        width: var(--space-20);
        flex-shrink: 0;
        font-size: var(--text-sm);
        font-weight: 500;
        color: var(--muted-foreground);
      }
      .tdd-prop-value {
        flex: 1;
        min-width: 0;
      }

      /* Description */
      .tdd-desc-view {
        position: relative;
        cursor: pointer;
        border-radius: var(--radius-md);
        padding: var(--space-4);
        margin: calc(var(--space-1) * -1);
        transition: background var(--dur-fast) var(--ease-1);
      }
      .tdd-desc-view:hover {
        background: color-mix(in oklch, var(--muted) 50%, transparent);
      }
      .tdd-desc-edit-badge {
        position: absolute;
        top: var(--space-2);
        right: var(--space-2);
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: var(--control-h-xs);
        height: var(--control-h-xs);
        border-radius: var(--radius-sm);
        color: var(--muted-foreground);
        opacity: 0;
        transition: opacity var(--dur-fast) var(--ease-1);
      }
      .tdd-desc-view:hover .tdd-desc-edit-badge {
        opacity: 1;
      }
      /* Read-mode editor renders its own content (shadow DOM). */
      .tdd-desc-md {
        font-size: var(--text-sm);
        line-height: 1.6;
        color: var(--foreground);
      }
      .tdd-desc-placeholder {
        cursor: pointer;
        border-radius: var(--radius-md);
        padding: var(--space-4);
        margin: calc(var(--space-1) * -1);
        font-size: var(--text-sm);
        color: var(--muted-foreground);
        transition: background var(--dur-fast) var(--ease-1);
      }
      .tdd-desc-placeholder:hover {
        background: color-mix(in oklch, var(--muted) 50%, transparent);
      }
      .tdd-desc-edit {
        display: flex;
        flex-direction: column;
      }

      /* Placeholder tabs */
      .tdd-placeholder {
        display: flex;
        align-items: center;
        justify-content: center;
        padding: var(--space-12);
        color: var(--muted-foreground);
        font-size: var(--text-sm);
      }

      /* Comments tab */
      .tdd-comments {
        display: flex;
        flex-direction: column;
        gap: var(--space-3);
      }

      /* Breadcrumb navigation */
      .tdd-breadcrumb {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-1) var(--space-6) 0;
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      .tdd-breadcrumb-back {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: var(--space-5);
        height: var(--space-5);
        border: none;
        border-radius: var(--radius-sm);
        background: transparent;
        color: var(--muted-foreground);
        cursor: pointer;
        transition: background var(--dur-fast) var(--ease-1);
      }
      .tdd-breadcrumb-back:hover {
        background: var(--accent);
        color: var(--foreground);
      }
      .tdd-breadcrumb-parent {
        cursor: pointer;
        color: var(--muted-foreground);
        transition: color var(--dur-fast) var(--ease-1);
      }
      .tdd-breadcrumb-parent:hover {
        color: var(--foreground);
        text-decoration: underline;
      }
      .tdd-breadcrumb-sep {
        color: var(--border);
      }
      .tdd-breadcrumb-current {
        color: var(--foreground);
        font-weight: 500;
      }

      /* Subtasks tab */
      .tdd-subtasks {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
      }
      .tdd-subtasks-head {
        display: flex;
        align-items: center;
      }
      .tdd-subtasks-progress {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      .tdd-subtask-list {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
        position: relative;
      }
      .tdd-subtask-row {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        width: 100%;
        /* Stable baseline so rows line up even when some have no
          assignees / a short title. */
        min-height: calc(var(--control-h) + var(--space-4));
        padding: var(--space-2);
        border: 1px solid var(--border);
        border-radius: var(--radius-md);
        background: var(--card);
        color: var(--foreground);
        font-size: var(--text-sm);
        text-align: left;
        cursor: pointer;
        transition: border-color var(--dur-fast) var(--ease-1);
      }
      .tdd-subtask-row:hover {
        border-color: var(--ring);
      }
      .tdd-subtask-status-dot {
        width: var(--space-2);
        height: var(--space-2);
        border-radius: var(--radius-full);
        background: var(--status-dot-color, var(--muted-foreground));
        flex-shrink: 0;
      }
      .tdd-subtask-title {
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .tdd-subtask-title.done {
        text-decoration: line-through;
        color: var(--muted-foreground);
      }
      .tdd-subtask-add {
        display: flex;
        gap: var(--space-2);
        margin-top: var(--space-1);
      }
      .tdd-subtask-input {
        flex: 1;
        height: var(--control-h-sm, 1.75rem);
        padding: 0 var(--space-2);
        border: 1px solid var(--input);
        border-radius: var(--radius-md);
        background: var(--background);
        color: var(--foreground);
        font-size: var(--text-sm);
        font-family: inherit;
        outline: none;
      }
      .tdd-subtask-input:focus {
        border-color: var(--ring);
        box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
      }

      /* Subtask row actions: status dropdown, assignees, delete */
      .tdd-subtask-actions {
        display: flex;
        align-items: center;
        gap: var(--space-1);
        flex-shrink: 0;
      }
      .tdd-subtask-status-select {
        padding: 0;
        border: none;
        background: transparent;
        color: var(--foreground);
        font-size: var(--text-2xs);
        font-family: inherit;
        cursor: pointer;
        outline: none;
        opacity: 0;
        transition: opacity var(--dur-fast) var(--ease-1);
        max-width: var(--space-20);
      }
      .tdd-subtask-row:hover .tdd-subtask-status-select {
        opacity: 1;
      }
      .tdd-subtask-assignees {
        display: flex;
        align-items: center;
        gap: 2px;
      }
      /* Compact assignee combobox inside a subtask row: hidden until the
        row is hovered, matching the old + affordance. */
      .tdd-subtask-assignee-picker {
        opacity: 0;
        transition: opacity var(--dur-fast) var(--ease-1);
      }
      .tdd-subtask-row:hover .tdd-subtask-assignee-picker {
        opacity: 1;
      }
      .tdd-subtask-del {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: var(--space-5);
        height: var(--space-5);
        border: none;
        border-radius: var(--radius-sm);
        background: transparent;
        color: var(--muted-foreground);
        cursor: pointer;
        opacity: 0;
        transition:
          opacity var(--dur-fast) var(--ease-1),
          background var(--dur-fast) var(--ease-1);
        flex-shrink: 0;
      }
      .tdd-subtask-row:hover .tdd-subtask-del {
        opacity: 1;
      }
      .tdd-subtask-del:hover {
        background: var(--accent);
        color: var(--destructive);
      }
      .tdd-subtask-inline-input {
        flex: 1;
        min-width: 0;
        height: var(--control-h-xs, 1.5rem);
        padding: 0 var(--space-1);
        border: 1px solid var(--ring);
        border-radius: var(--radius-sm);
        background: var(--background);
        color: var(--foreground);
        font-size: var(--text-sm);
        font-family: inherit;
        outline: none;
      }
      .tdd-subtask-controls-row {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        margin-left: auto;
      }
      .tdd-subtask-collapse-btn {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: var(--space-5);
        height: var(--space-5);
        border: none;
        border-radius: var(--radius-sm);
        background: transparent;
        color: var(--muted-foreground);
        cursor: pointer;
        font-size: var(--text-sm);
        transition: color var(--dur-fast) var(--ease-1);
      }
      .tdd-subtask-collapse-btn:hover {
        color: var(--foreground);
      }
      .tdd-subtask-collapse-btn.collapsed {
        transform: rotate(-90deg);
      }
      .tdd-subtask-hide-check {
        display: inline-flex;
        align-items: center;
        gap: var(--space-1);
        font-size: var(--text-xs);
        color: var(--muted-foreground);
        cursor: pointer;
        user-select: none;
      }
      .tdd-subtask-hide-check input {
        accent-color: var(--primary);
      }
      /* Grip handle for drag reorder */
      .tdd-subtask-grip {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: var(--space-4);
        height: var(--space-5);
        border: none;
        border-radius: var(--radius-sm);
        background: transparent;
        color: var(--muted-foreground);
        cursor: grab;
        flex-shrink: 0;
        padding: 0;
        opacity: 0;
        transition:
          opacity var(--dur-fast) var(--ease-1),
          color var(--dur-fast) var(--ease-1);
      }
      .tdd-subtask-row:hover .tdd-subtask-grip {
        opacity: 1;
      }
      .tdd-subtask-grip:active {
        cursor: grabbing;
      }
      .tdd-subtask-grip:hover {
        color: var(--foreground);
      }

      /* Drop indicator line */
      .tdd-indicator {
        position: absolute;
        left: 0;
        right: 0;
        height: var(--space-0-5);
        display: none;
        align-items: center;
        padding: 0 var(--space-1);
        pointer-events: none;
        z-index: var(--z-sticky);
      }
      .tdd-indicator-line {
        height: var(--space-0-5);
        flex: 1;
        background: var(--primary);
        border-radius: var(--space-0-5);
      }
      .tdd-indicator-dot {
        width: var(--space-1-5);
        height: var(--space-1-5);
        border-radius: var(--radius-full);
        background: var(--primary);
        flex-shrink: 0;
      }

      /* Container highlight during drag */
      .tdd-subtask-list[data-over] {
        background: color-mix(in oklch, var(--primary) 5%, transparent);
        box-shadow: inset 0 0 0 1px
          color-mix(in oklch, var(--primary) 20%, transparent);
        border-radius: var(--radius-lg);
      }

      /* Dragging row */
      .tdd-subtask-row[data-dragging] {
        opacity: 0.4;
      }

      /* subtask row status popover trigger (compact: dot + chevron) */
      .tdd-subtask-sel-trigger {
        display: inline-flex;
        align-items: center;
        gap: 2px;
        padding: 0;
        border: none;
        background: transparent;
        cursor: pointer;
        flex-shrink: 0;
      }
      .tdd-subtask-sel-trigger:focus-visible {
        outline: 2px solid var(--ring);
        outline-offset: 2px;
        border-radius: var(--radius-sm);
      }
      .tdd-subtask-sel-dot {
        width: var(--space-2);
        height: var(--space-2);
        border-radius: var(--radius-full);
        background: var(--status-dot-color, var(--muted-foreground));
        flex-shrink: 0;
      }
      .tdd-subtask-sel-trigger plume-icon {
        color: var(--muted-foreground);
        opacity: 0.5;
        flex-shrink: 0;
      }

      /* Status popover menu */
      .pop {
        min-width: var(--space-40);
        max-height: var(--space-56);
        overflow-y: auto;
      }
      .opt {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        width: 100%;
        padding: var(--space-1-5) var(--space-2);
        border: none;
        border-radius: var(--radius-sm);
        background: transparent;
        color: var(--foreground);
        font-size: var(--text-sm);
        font-family: inherit;
        text-align: left;
        cursor: pointer;
        white-space: nowrap;
      }
      .opt:hover {
        background: var(--accent);
      }
      .opt .dot {
        width: var(--space-2-5);
        height: var(--space-2-5);
        border-radius: var(--radius-full);
        flex-shrink: 0;
      }
      .opt .name {
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
      }
      .opt plume-icon.check {
        color: var(--primary);
        flex-shrink: 0;
      }

      /* Subtask assignee popover with combobox */
      .tdd-subtask-assignee-pop {
        min-width: var(--space-48);
        padding: var(--space-1);
      }

      .tdd-subtask-empty {
        font-size: var(--text-sm);
        color: var(--muted-foreground);
        text-align: center;
        padding: var(--space-8) 0;
      }

      .tdd-deps {
        display: flex;
        flex-direction: column;
        gap: var(--space-5);
      }
      .tdd-dep-section {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
      }
      .tdd-dep-head {
        display: flex;
        align-items: center;
        justify-content: space-between;
      }
      .tdd-dep-heading {
        font-size: var(--text-xs);
        font-weight: 600;
        color: var(--muted-foreground);
        text-transform: uppercase;
        letter-spacing: 0.04em;
      }
      .tdd-dep-list {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
      }
      .tdd-dep-row {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-2);
        border: 1px solid var(--border);
        border-radius: var(--radius-md);
        background: var(--card);
      }
      .tdd-dep-dot {
        width: var(--space-2);
        height: var(--space-2);
        border-radius: var(--radius-full);
        background: var(--status-dot-color, var(--muted-foreground));
        flex-shrink: 0;
      }
      .tdd-dep-title {
        flex: 1;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        text-align: left;
        border: none;
        background: transparent;
        color: var(--foreground);
        font-size: var(--text-sm);
        cursor: pointer;
        padding: 0;
      }
      .tdd-dep-title:hover {
        text-decoration: underline;
      }
      .tdd-dep-done {
        color: var(--primary);
        display: inline-flex;
      }
      .tdd-dep-empty {
        font-size: var(--text-sm);
        color: var(--muted-foreground);
        padding: var(--space-2) 0;
      }
      .tdd-dep-add {
        display: flex;
        gap: var(--space-2);
        align-items: center;
      }
      .tdd-dep-add-trigger {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        flex: 1;
        min-width: 0;
        height: var(--control-h-sm, 1.75rem);
        padding: 0 var(--space-2);
        border: 1px solid var(--input);
        border-radius: var(--radius-md);
        background: var(--background);
        color: var(--foreground);
        font-size: var(--text-sm);
        font-family: inherit;
        cursor: pointer;
      }
      .tdd-dep-add-trigger:hover {
        background: var(--accent);
      }
      .tdd-dep-add-label {
        flex: 1;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        color: var(--muted-foreground);
      }
      .tdd-dep-search-pop {
        width: var(--space-72);
        max-height: var(--space-80);
        overflow: hidden;
        display: flex;
        flex-direction: column;
      }
      .tdd-dep-search {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-2) var(--space-3);
        border-bottom: 1px solid var(--border);
      }
      .tdd-dep-search input {
        flex: 1;
        border: none;
        outline: none;
        background: transparent;
        color: inherit;
        font-size: var(--text-sm);
        font-family: inherit;
      }
      .tdd-dep-search-list {
        max-height: var(--space-64);
        overflow-y: auto;
        padding: var(--space-1);
        display: flex;
        flex-direction: column;
        gap: var(--space-0-5);
      }
      .tdd-dep-search-opt {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        width: 100%;
        padding: var(--space-1-5) var(--space-2);
        border: none;
        border-radius: var(--radius-sm);
        background: transparent;
        color: inherit;
        font-size: var(--text-sm);
        font-family: inherit;
        text-align: left;
        cursor: pointer;
      }
      .tdd-dep-search-opt:hover,
      .tdd-dep-search-opt.selected {
        background: var(--accent);
      }
      .tdd-dep-search-dot {
        width: var(--space-2);
        height: var(--space-2);
        border-radius: var(--radius-full);
        flex-shrink: 0;
      }
      .tdd-dep-search-name {
        flex: 1;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .tdd-dep-search-empty {
        padding: var(--space-3);
        text-align: center;
        color: var(--muted-foreground);
        font-size: var(--text-sm);
      }
      /* Time tracking tab */
      .tdd-time {
        display: flex;
        flex-direction: column;
        gap: var(--space-3);
      }
      .tdd-time-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
      }
      .tdd-time-header-label {
        font-size: var(--text-xs);
        font-weight: 500;
        color: var(--muted-foreground);
      }
      .tdd-time-header-total {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      .tdd-timer-active {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        border: 1px solid var(--border);
        border-radius: var(--radius-md);
        background: color-mix(in oklch, var(--muted) 30%, transparent);
        padding: var(--space-2) var(--space-3);
      }
      .tdd-timer-icon {
        color: var(--primary);
        flex-shrink: 0;
      }
      .tdd-timer-active-icon {
        animation: tdd-pulse var(--dur-slow) ease-in-out infinite;
      }
      @keyframes tdd-pulse {
        0%,
        100% {
          opacity: 1;
        }
        50% {
          opacity: 0.5;
        }
      }
      .tdd-timer-display {
        flex: 1;
        font-size: var(--text-sm);
        font-weight: 500;
        font-variant-numeric: tabular-nums;
      }
      .tdd-timer-desc {
        max-width: var(--space-32);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      .tdd-timer-stop {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: var(--space-6);
        height: var(--space-6);
        border: none;
        border-radius: var(--radius-sm);
        background: transparent;
        color: var(--destructive);
        cursor: pointer;
        transition: background var(--dur-fast) var(--ease-1);
        flex-shrink: 0;
      }
      .tdd-timer-stop:hover {
        background: color-mix(in oklch, var(--destructive) 10%, transparent);
      }
      .tdd-time-section-label {
        font-size: var(--text-xs);
        font-weight: 500;
        color: var(--muted-foreground);
      }
      .tdd-time-section-hint {
        font-size: var(--text-2xs);
        color: color-mix(in oklch, var(--muted-foreground) 60%, transparent);
      }
      .tdd-timer-start {
        display: flex;
        align-items: center;
        gap: var(--space-2);
      }
      .tdd-timer-start-input {
        flex: 1;
        height: var(--control-h);
        padding: 0 var(--space-3);
        border: 1px solid var(--input);
        border-radius: var(--radius-md);
        background: var(--background);
        color: var(--foreground);
        font-size: var(--text-sm);
        font-family: inherit;
        outline: none;
        transition:
          border-color var(--dur-fast) var(--ease-1),
          box-shadow var(--dur-fast) var(--ease-1);
      }
      .tdd-timer-start-input:focus {
        border-color: var(--ring);
        box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
      }
      .tdd-manual {
        display: flex;
        align-items: center;
        gap: var(--space-2);
      }
      .tdd-manual-input {
        flex: 1;
        height: var(--control-h);
        padding: 0 var(--space-3);
        border: 1px solid var(--input);
        border-radius: var(--radius-md);
        background: var(--background);
        color: var(--foreground);
        font-size: var(--text-sm);
        font-family: inherit;
        outline: none;
        transition:
          border-color var(--dur-fast) var(--ease-1),
          box-shadow var(--dur-fast) var(--ease-1);
      }
      .tdd-manual-input:focus {
        border-color: var(--ring);
        box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
      }
      .tdd-manual-num {
        width: var(--space-20);
        height: var(--control-h);
        padding: 0 var(--space-2);
        border: 1px solid var(--input);
        border-radius: var(--radius-md);
        background: var(--background);
        color: var(--foreground);
        font-size: var(--text-sm);
        font-family: inherit;
        text-align: center;
        outline: none;
        transition:
          border-color var(--dur-fast) var(--ease-1),
          box-shadow var(--dur-fast) var(--ease-1);
        -moz-appearance: textfield;
      }
      .tdd-manual-num::-webkit-outer-spin-button,
      .tdd-manual-num::-webkit-inner-spin-button {
        -webkit-appearance: none;
        margin: 0;
      }
      .tdd-manual-num:focus {
        border-color: var(--ring);
        box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
      }
      .tdd-time-sep {
        height: 1px;
        background: var(--border);
        margin: 0;
        border: none;
      }
      .tdd-entry-list {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
      }
      .tdd-entry {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: var(--space-1) var(--space-2);
        border-radius: var(--radius-md);
        font-size: var(--text-sm);
        transition: background var(--dur-fast) var(--ease-1);
      }
      .tdd-entry:hover {
        background: color-mix(in oklch, var(--muted) 50%, transparent);
      }
      .tdd-entry-desc {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        flex: 1;
      }
      .tdd-entry-right {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        flex-shrink: 0;
      }
      .tdd-entry-dur {
        font-variant-numeric: tabular-nums;
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      .tdd-entry-del {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: var(--space-5);
        height: var(--space-5);
        border: none;
        border-radius: var(--radius-sm);
        background: transparent;
        color: var(--muted-foreground);
        cursor: pointer;
        opacity: 0;
        transition:
          opacity var(--dur-fast) var(--ease-1),
          background var(--dur-fast) var(--ease-1);
        flex-shrink: 0;
      }
      .tdd-entry:hover .tdd-entry-del {
        opacity: 1;
      }
      .tdd-entry-del:hover {
        background: var(--accent);
        color: var(--destructive);
      }
      .tdd-time-error {
        font-size: var(--text-xs);
        color: var(--destructive);
      }

      /* Activity tab */
      .tdd-activity-list {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
        list-style: none;
        margin: 0;
        padding: 0;
      }
      .tdd-activity-row {
        display: flex;
        gap: var(--space-2);
        padding: var(--space-2) 0;
        border-bottom: 1px solid var(--border);
      }
      .tdd-activity-icon {
        color: var(--muted-foreground);
      }
      .tdd-activity-body {
        display: flex;
        flex-wrap: wrap;
        align-items: baseline;
        gap: var(--space-1);
        font-size: var(--text-sm);
      }
      .tdd-activity-actor {
        font-weight: 600;
      }
      .tdd-activity-label {
        color: var(--muted-foreground);
      }
      .tdd-activity-values {
        display: inline-flex;
        gap: var(--space-1);
        font-variant-numeric: tabular-nums;
      }
      .tdd-activity-new {
        color: var(--primary);
      }
      .tdd-activity-time {
        width: 100%;
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      .tdd-activity-more {
        text-align: center;
        padding-top: var(--space-2);
      }
      .tdd-load-more {
        background: none;
        border: none;
        color: var(--primary);
        cursor: pointer;
        font-size: var(--text-sm);
        padding: var(--space-1) var(--space-3);
        border-radius: var(--radius);
        transition: background var(--dur-fast);
      }
      .tdd-load-more:hover {
        background: var(--accent);
      }
      .tdd-load-more:disabled {
        opacity: 0.5;
        cursor: default;
      }

      /* Attachments tab */
      .tdd-attach {
        display: flex;
        flex-direction: column;
        gap: var(--space-3);
      }
      .tdd-attach-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
      }
      .tdd-attach-header-label {
        font-size: var(--text-xs);
        font-weight: 500;
        color: var(--muted-foreground);
      }
      .tdd-attach-empty {
        font-size: var(--text-xs);
        color: color-mix(in oklch, var(--muted-foreground) 60%, transparent);
      }
      .tdd-attach-file {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-1-5) var(--space-1);
        border-radius: var(--radius-md);
        transition: background var(--dur-fast) var(--ease-1);
      }
      .tdd-attach-file:hover {
        background: color-mix(in oklch, var(--muted) 50%, transparent);
      }
      .tdd-attach-file-icon {
        color: var(--muted-foreground);
        flex-shrink: 0;
      }
      .tdd-attach-file-info {
        flex: 1;
        min-width: 0;
        display: flex;
        flex-direction: column;
      }
      .tdd-attach-file-name {
        font-size: var(--text-xs);
        font-weight: 500;
        color: var(--foreground);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .tdd-attach-file-name:hover {
        text-decoration: underline;
      }
      .tdd-attach-file-meta {
        font-size: var(--text-2xs);
        color: var(--muted-foreground);
      }
      .tdd-attach-file-del {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: var(--space-5);
        height: var(--space-5);
        border: none;
        border-radius: var(--radius-sm);
        background: transparent;
        color: var(--muted-foreground);
        cursor: pointer;
        opacity: 0;
        transition:
          opacity var(--dur-fast) var(--ease-1),
          background var(--dur-fast) var(--ease-1);
        flex-shrink: 0;
      }
      .tdd-attach-file:hover .tdd-attach-file-del {
        opacity: 1;
      }
      .tdd-attach-file-del:hover {
        background: var(--accent);
        color: var(--destructive);
      }

      /* Delete confirmation overlay */
      .tdd-confirm-overlay {
        position: absolute;
        inset: 0;
        z-index: var(--z-overlay);
        display: flex;
        align-items: center;
        justify-content: center;
        background: color-mix(in oklch, var(--popover) 70%, transparent);
        border-radius: var(--radius-lg);
      }
      .tdd-confirm-box {
        width: var(--space-96);
        max-width: calc(100% - var(--space-8));
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
        background: var(--popover);
        box-shadow: var(--shadow-lg);
        padding: var(--space-6);
      }
      .tdd-confirm-box h3 {
        margin: 0 0 var(--space-2);
        font-size: var(--text-lg);
        font-weight: 600;
      }
      .tdd-confirm-box p {
        margin: 0 0 var(--space-5);
        font-size: var(--text-sm);
        color: var(--muted-foreground);
      }
      .tdd-confirm-actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
      }
      .tdd-move-box {
        width: var(--space-112);
      }
      .tdd-move-form {
        display: flex;
        flex-direction: column;
        gap: var(--space-3);
        margin-bottom: var(--space-5);
      }
      .tdd-move-hint {
        margin-bottom: var(--space-4) !important;
      }
      .tdd-flex-col {
        display: flex;
        flex-direction: column;
      }
      .tdd-flex-col-gap-1 {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
      }
      .tdd-flex-col-gap-2 {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
      }
      .tdd-flex-between {
        display: flex;
        align-items: center;
        justify-content: space-between;
      }
      .tdd-flex-1 {
        flex: 1;
      }
      .tdd-attach-btn {
        height: var(--space-7);
        gap: var(--space-1-5);
        font-size: var(--text-xs);
      }
      .tdd-file-input {
        display: none;
      }
    `,
  ];

  @property({ type: Object, attribute: false })
  task: DtoTaskResponse | null = null;

  @property({ type: Object, attribute: false })
  project: DtoProjectResponse | null = null;

  @property({ type: Array, attribute: false })
  statuses: DtoTaskStatusResponse[] = [];

  @state()
  private _editingTitle = false;
  @state()
  private _titleDraft = "";
  @state()
  private _editingDesc = false;
  @state()
  private _descDraft = "";
  @state()
  private _tab = "description";
  @state()
  private _menuOpen = false;
  @state()
  private _confirmDelete = false;
  @state()
  private _members: DtoProjectMemberResponse[] = [];
  @state()
  private _cycles: DtoCycleResponse[] = [];
  @state()
  private _loadedProjectId = "";

  @query("plume-task-editor")
  private _taskEditor!: PlumeTaskEditor | null;
  @query("#tdd-file-input")
  private _fileInput!: HTMLInputElement | null;

  // Subtasks state
  @state()
  private _subtasks: DtoTaskResponse[] = [];
  @state()
  private _subtaskTitle = "";
  @state()
  private _creatingSubtask = false;
  @state()
  private _editingSubtaskId = "";
  @state()
  private _editingSubtaskTitle = "";
  @state()
  private _confirmDeleteSubtaskId = "";
  @state()
  private _deletingSubtask = false;
  @state()
  private _hideCompletedSubtasks = false;
  @state()
  private _subtasksCollapsed = false;

  // DnD reorder refs and state
  #subtaskListRef = createRef<HTMLDivElement>();
  #subtaskIndicatorRef = createRef<HTMLDivElement>();
  #draggingId: string | null = null;

  // Breadcrumb navigation stack (task IDs for parent navigation)
  private _taskStack: string[] = [];
  #signals = new SignalController(this);

  // Move-to-project state
  @state()
  private _showMoveModal = false;
  @state()
  private _moveProjectId = "";
  @state()
  private _moving = false;

  // Dependencies state
  @state()
  private _blocking: DtoTaskResponse[] = [];
  @state()
  private _blocked: DtoTaskResponse[] = [];
  @state()
  private _depAddOpen = false;
  @state()
  private _depAddId = "";
  @state()
  private _depSearch = "";

  // Activity state
  @state()
  private _activity: DtoTaskActivityResponse[] = [];
  @state()
  private _activityLoading = false;
  @state()
  private _activityError = "";
  @state()
  private _activityHasMore = false;
  @state()
  private _activityLoadingMore = false;
  private _activityCursor: string | undefined = undefined;

  // WebSocket live-update handler: reloads activity when another user
  // records activity on this task.
  private _activityRecordedHandler!: (e: Event) => void;

  // Time tracking state
  @state()
  private _timeEntries: DtoTimeEntryResponse[] = [];
  @state()
  private _timerElapsed = 0;
  @state()
  private _timerDesc = "";
  @state()
  private _manualDesc = "";
  @state()
  private _manualHours = "";
  @state()
  private _manualMinutes = "";
  @state()
  private _timeError = "";
  private _timerInterval: ReturnType<typeof setInterval> | null = null;

  // Attachments state
  @state()
  private _attachments: DtoAttachmentResponse[] = [];
  @state()
  private _uploading = false;

  // Actions menu
  private _menuOutsideClick = new OutsideClickController(this, () => {
    this._menuOpen = false;
  });
  private _onMenuKeydown = (e: KeyboardEvent) => {
    if (e.key === "Escape") this._menuOpen = false;
  };

  protected updated(changedProps: Map<string, unknown>) {
    if (changedProps.has("_menuOpen")) {
      if (this._menuOpen) {
        this._menuOutsideClick.connect();
        document.addEventListener("keydown", this._onMenuKeydown);
      } else {
        this._menuOutsideClick.disconnect();
        document.removeEventListener("keydown", this._onMenuKeydown);
      }
    }
    if (changedProps.has("_editingDesc")) {
      if (this._editingDesc) {
        // Use pointerdown (not click) so we catch the interaction before
        // focus moves: composedPath() pierces the editor's shadow DOM.
        document.addEventListener("pointerdown", this._onDescOutsideDown);
      } else {
        document.removeEventListener("pointerdown", this._onDescOutsideDown);
      }
    }
    if (changedProps.has("task") && this.task) {
      this._titleDraft = this.task.title ?? "";
      this._descDraft = this.task.description ?? "";
      this._editingTitle = false;
      this._editingDesc = false;
      this._editingSubtaskId = "";
      this._taskStack = [];
      this._loadData();
      this._loadSubtasks();
      this._loadTimeEntries();
      this._loadAttachments();
      this._loadDependencies();
      this._loadActivity();
    }
  }

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(projectDetail, projectsSignal);
    this._activityRecordedHandler = (e: Event) => {
      const custom = e as CustomEvent<{ taskId: string }>;
      if (custom.detail?.taskId === this.task?.id) {
        void this._loadActivity();
      }
    };
    document.addEventListener(
      "plume-task-activity-recorded",
      this._activityRecordedHandler,
    );
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this._menuOutsideClick.disconnect();
    document.removeEventListener("keydown", this._onMenuKeydown);
    document.removeEventListener("pointerdown", this._onDescOutsideDown);
    document.removeEventListener(
      "plume-task-activity-recorded",
      this._activityRecordedHandler,
    );
    if (this._timerInterval) {
      clearInterval(this._timerInterval);
      this._timerInterval = null;
    }
  }

  private async _loadData() {
    const pid = this.project?.id;
    if (!pid || pid === this._loadedProjectId) return;
    this._loadedProjectId = pid;

    const hasCycles = (this.project?.cycle_duration ?? 0) > 0;
    const [membersRes, cyclesRes] = await Promise.all([
      getProjectsByIdMembers({ path: { id: pid }, throwOnError: true })
        .catch(() => null),
      hasCycles
        ? getProjectsByIdCycles({ path: { id: pid }, throwOnError: true })
          .catch(() => null)
        : Promise.resolve(null),
    ]);

    this._members = membersRes?.data?.items ?? [];
    this._cycles = (cyclesRes?.data) ?? [];
  }

  // Subtask data loading
  private async _loadSubtasks() {
    const pid = this.project?.id;
    const tid = this.task?.id;
    if (!pid || !tid) return;
    try {
      const { data } = await getProjectsByIdTasksByTaskIdSubtasks({
        path: { id: pid, taskId: tid },
        throwOnError: true,
      });
      this._subtasks = data ?? [];
    } catch {
      this._subtasks = [];
    }
  }

  // Breadcrumb navigation: navigate to parent task
  private _navigateToParent(parentId: string | undefined) {
    if (!parentId) return;
    // Push current task onto stack
    if (this.task?.id) {
      this._taskStack = [...this._taskStack, this.task.id];
    }
    selectTask(parentId);
    this._tab = "description";
  }

  private _navigateBack() {
    const prevId = this._taskStack.pop();
    if (prevId) {
      selectTask(prevId);
      this._tab = "subtasks";
    }
  }

  // Time entry data loading
  private async _loadTimeEntries() {
    const pid = this.project?.id;
    const tid = this.task?.id;
    if (!pid || !tid) return;
    try {
      const { data } = await getProjectsByIdTasksByTaskIdTimeEntries({
        path: { id: pid, taskId: tid },
        throwOnError: true,
      });
      this._timeEntries = data ?? [];
      this._startTimerIfActive();
    } catch {
      this._timeEntries = [];
    }
  }

  private async _startTimer() {
    const pid = this.project?.id;
    const tid = this.task?.id;
    if (!pid || !tid) return;
    try {
      await postProjectsByIdTasksByTaskIdTimeEntriesStart({
        path: { id: pid, taskId: tid },
        body: { description: this._timerDesc || undefined },
        throwOnError: true,
      });
      this._timerDesc = "";
      await this._loadTimeEntries();
    } catch {
      showToast(msg("Failed to start timer"), { variant: "error" });
    }
  }

  private async _stopTimer() {
    const pid = this.project?.id;
    const tid = this.task?.id;
    if (!pid || !tid) return;
    try {
      await postProjectsByIdTasksByTaskIdTimeEntriesStop({
        path: { id: pid, taskId: tid },
        throwOnError: true,
      });
      this._clearTimerInterval();
      await this._loadTimeEntries();
    } catch {
      showToast(msg("Failed to stop timer"), { variant: "error" });
    }
  }

  private async _addManualTimeEntry() {
    const pid = this.project?.id;
    const tid = this.task?.id;
    const h = Number(this._manualHours) || 0;
    const m = Number(this._manualMinutes) || 0;
    const totalMinutes = h * 60 + m;
    if (!pid || !tid || !this._manualDesc.trim() || totalMinutes <= 0) return;
    this._timeError = "";
    try {
      await postProjectsByIdTasksByTaskIdTimeEntries({
        path: { id: pid, taskId: tid },
        body: {
          description: this._manualDesc.trim(),
          duration_minutes: totalMinutes,
        },
        throwOnError: true,
      });
      this._manualDesc = "";
      this._manualHours = "";
      this._manualMinutes = "";
      await this._loadTimeEntries();
      void this._loadActivity();
    } catch {
      this._timeError = msg("Failed to add time entry");
    }
  }

  private async _deleteTimeEntry(entryId: string) {
    const pid = this.project?.id;
    const tid = this.task?.id;
    if (!pid || !tid) return;
    try {
      await deleteProjectsByIdTasksByTaskIdTimeEntriesByEntryId({
        path: { id: pid, taskId: tid, entryId },
        throwOnError: true,
      });
      await this._loadTimeEntries();
    } catch {
      showToast(msg("Failed to delete time entry"), { variant: "error" });
    }
  }

  private _startTimerIfActive() {
    this._clearTimerInterval();
    const activeEntry = this._timeEntries.find((e) => !e.ended_at);
    if (activeEntry?.started_at) {
      const startTime = new Date(activeEntry.started_at).getTime();
      const tick = () => {
        this._timerElapsed = Math.max(
          0,
          Math.floor((Date.now() - startTime) / 1000),
        );
      };
      tick();
      this._timerInterval = setInterval(tick, 1000);
    }
  }

  private _clearTimerInterval() {
    if (this._timerInterval) {
      clearInterval(this._timerInterval);
      this._timerInterval = null;
    }
    this._timerElapsed = 0;
  }

  // Attachment data loading
  private async _loadAttachments() {
    const pid = this.project?.id;
    const tid = this.task?.id;
    if (!pid || !tid) return;
    try {
      const { data } = await getProjectsByIdTasksByTaskIdAttachments({
        path: { id: pid, taskId: tid },
        throwOnError: true,
      });
      this._attachments = data ?? [];
    } catch {
      this._attachments = [];
    }
  }

  private async _uploadAttachment(file: File) {
    const pid = this.project?.id;
    const tid = this.task?.id;
    if (!pid || !tid) return;
    this._uploading = true;
    try {
      await postProjectsByIdTasksByTaskIdAttachments({
        path: { id: pid, taskId: tid },
        body: { file },
        throwOnError: true,
      });
      await this._loadAttachments();
      void this._loadActivity();
    } catch {
      showToast(msg("Failed to upload attachment"), { variant: "error" });
    } finally {
      this._uploading = false;
    }
  }

  private async _deleteAttachment(attachmentId: string) {
    const pid = this.project?.id;
    const tid = this.task?.id;
    if (!pid || !tid) return;
    try {
      await deleteProjectsByIdTasksByTaskIdAttachmentsByAttachmentId({
        path: { id: pid, taskId: tid, attachmentId },
        throwOnError: true,
      });
      await this._loadAttachments();
      void this._loadActivity();
    } catch {
      showToast(msg("Failed to delete attachment"), { variant: "error" });
    }
  }

  // Title editing
  private _startTitleEdit() {
    this._titleDraft = this.task?.title ?? "";
    this._editingTitle = true;
  }

  private _saveTitle() {
    const v = this._titleDraft.trim();
    if (v && v !== this.task?.title) {
      this._updateField("title", v);
    }
    this._editingTitle = false;
  }

  private _cancelTitle() {
    this._editingTitle = false;
  }

  // Description editing
  private _startDescEdit() {
    this._descDraft = this.task?.description ?? "";
    this._editingDesc = true;
  }

  private _onDescChange(e: CustomEvent) {
    this._descDraft = (e.detail.value as string) ?? "";
  }

  private _saveDesc() {
    if (this._descDraft !== (this.task?.description ?? "")) {
      this._updateField("description", this._descDraft);
    }
    this._editingDesc = false;
  }

  private _cancelDesc() {
    this._descDraft = this.task?.description ?? "";
    this._editingDesc = false;
  }

  /**
   * Click-outside-to-save. Uses pointerdown + composedPath() so it reliably
   * detects clicks inside the editor's shadow DOM (ProseMirror content +
   * bubble menu) without the focus-race issues that plague focusout.
   */
  private _onDescOutsideDown = (e: PointerEvent) => {
    if (this._taskEditor && e.composedPath().includes(this._taskEditor)) return;
    this._saveDesc();
  };

  // Property updates
  private async _updateField(field: string, value: unknown) {
    if (!this.task?.id || !this.project?.id) return;
    try {
      await updateTask(
        this.project.id,
        this.task.id,
        { [field]: value } as Record<
          string,
          unknown
        >,
      );
      void this._loadActivity();
    } catch (err) {
      logError("updateField failed:", err);
    }
  }

  private _onStatusChange(id: string) {
    this._updateField("status_id", id);
  }

  private _onPriorityChange(p: string) {
    this._updateField("priority", p);
  }

  private _onAssigneesChange(ids: string[]) {
    this._updateField("assignee_ids", ids);
  }

  private async _onLabelsChange(labelIds: string[]): Promise<void> {
    if (!this.task?.id || !this.project?.id) return;
    try {
      await setTaskLabels(this.project.id, this.task.id, labelIds);
      void this._loadActivity();
    } catch (err) {
      logError("onLabelsChange failed:", err);
    }
  }

  private _onStartDateChange(v: string) {
    // Empty string clears the field (backend update contract: "" = clear,
    // null = leave unchanged). Pass the ISO through as-is when set.
    this._updateField("started_at", v ? new Date(v).toISOString() : "");
  }

  private _onDueDateChange(v: string) {
    this._updateField("due_at", v ? new Date(v).toISOString() : "");
  }

  private _onCycleChange(id: string) {
    this._updateField("cycle_id", id || null);
  }

  private _onEstimateChange(v: string) {
    const num = v ? Number(v) : null;
    this._updateField("estimate", num);
  }

  // Delete
  private _promptDelete() {
    this._menuOpen = false;
    this._confirmDelete = true;
  }

  private async _confirmDeleteTask() {
    if (!this.task?.id || !this.project?.id) return;
    await deleteTask(this.project.id, this.task.id);
    this._confirmDelete = false;
    this.dispatchEvent(
      new CustomEvent("delete", {
        detail: this.task.id,
        bubbles: true,
        composed: true,
      }),
    );
    this._close();
  }

  private async _confirmDeleteSubtask() {
    const id = this._confirmDeleteSubtaskId;
    if (!id || !this.project?.id) return;
    this._deletingSubtask = true;
    try {
      await deleteTask(this.project.id, id);
      this._confirmDeleteSubtaskId = "";
      await this._loadSubtasks();
    } catch {
      // showToast handled by deleteTask store helper on error
    } finally {
      this._deletingSubtask = false;
    }
  }

  private _cancelDelete() {
    this._confirmDelete = false;
  }

  // Duplicate / Move
  private async _duplicateTask(includeSubtasks = false): Promise<void> {
    if (!this.task?.id || !this.project?.id) return;
    this._menuOpen = false;
    const dup = await duplicateTask(
      this.project.id,
      this.task.id,
      includeSubtasks,
    );
    if (dup) {
      showToast(
        includeSubtasks ? "Task + subtasks duplicated" : "Task duplicated",
        { variant: "success" },
      );
      void this._loadActivity();
    } else {
      showToast(msg("Failed to duplicate task"), { variant: "error" });
    }
  }

  // Promote a subtask to a top-level task (clear its parent).
  private async _promoteToTopLevel(): Promise<void> {
    if (!this.task?.id || !this.project?.id || !this.task.parent_task_id) {
      return;
    }
    this._menuOpen = false;
    try {
      await updateTask(this.project.id, this.task.id, { parent_task_id: "" });
      showToast(msg("Promoted to top-level task"), { variant: "success" });
      void this._loadActivity();
    } catch (err) {
      logError("promoteToTopLevel failed:", err);
      showToast(msg("Failed to promote task"), { variant: "error" });
    }
  }

  #copyAsMarkdown(): void {
    this._menuOpen = false;
    const t = this.task;
    if (!t) return;
    const lines: string[] = [`# ${t.title}`];
    if (t.description) {
      lines.push("");
      lines.push(t.description);
    }
    lines.push("");
    lines.push(`**Status:** ${t.status_id}  `);
    lines.push(`**Priority:** ${t.priority}  `);
    if (t.estimate) {
      lines.push(`**Estimate:** ${t.estimate}  `);
    }
    if (t.started_at) {
      lines.push(`**Started:** ${t.started_at}  `);
    }
    if (t.due_at) {
      lines.push(`**Due:** ${t.due_at}  `);
    }
    if (t.completed_at) {
      lines.push(`**Completed:** ${t.completed_at}  `);
    }
    const md = lines.join("\n");
    navigator.clipboard.writeText(md).then(
      () => showToast(msg("Copied as Markdown"), { variant: "success" }),
      () => showToast(msg("Failed to copy"), { variant: "error" }),
    );
  }

  private _openMoveModal(): void {
    if (!this.task?.id || !this.project?.id) return;
    this._menuOpen = false;
    this._moveProjectId = "";
    this._showMoveModal = true;
  }

  private async _confirmMove(): Promise<void> {
    if (!this.task?.id || !this.project?.id || !this._moveProjectId) {
      return;
    }
    this._moving = true;
    // to_status_id omitted: the backend assigns the target project's default
    // status so the caller doesn't need to pre-load the target's statuses.
    const ok = await moveTaskToProject(this.project.id, this.task.id, {
      to_project_id: this._moveProjectId,
    });
    this._moving = false;
    if (ok) {
      this._showMoveModal = false;
      showToast(msg("Task moved"), { variant: "success" });
      this._close();
    } else {
      showToast(msg("Failed to move task"), { variant: "error" });
    }
  }

  // Dependencies
  private async _loadDependencies(): Promise<void> {
    const pid = this.project?.id;
    const tid = this.task?.id;
    if (!pid || !tid) return;
    const { blocking, blocked } = await fetchTaskDependencies(pid, tid);
    this._blocking = blocking;
    this._blocked = blocked;
  }

  /** Load the first page of activity (resets cursor). */
  private async _loadActivity(): Promise<void> {
    const pid = this.project?.id;
    const tid = this.task?.id;
    if (!pid || !tid) return;
    this._activityLoading = true;
    this._activityError = "";
    this._activityCursor = undefined;
    this._activityHasMore = false;
    try {
      const { data } = await getProjectsByIdTasksByTaskIdActivity({
        path: { id: pid, taskId: tid },
        query: { limit: 20 },
        throwOnError: true,
      });
      this._activity = data?.items ?? [];
      this._activityHasMore = data?.has_more ?? false;
      this._activityCursor = data?.next_cursor || undefined;
    } catch (err) {
      logError("loadActivity failed:", err);
      this._activity = [];
      this._activityHasMore = false;
      this._activityCursor = undefined;
      this._activityError = msg("Failed to load activity.");
    } finally {
      this._activityLoading = false;
    }
  }

  /** Append the next page of activity using the stored cursor. */
  private async _loadMoreActivity(): Promise<void> {
    if (!this._activityCursor || this._activityLoadingMore) return;
    const pid = this.project?.id;
    const tid = this.task?.id;
    if (!pid || !tid) return;
    this._activityLoadingMore = true;
    try {
      const { data } = await getProjectsByIdTasksByTaskIdActivity({
        path: { id: pid, taskId: tid },
        query: { cursor: this._activityCursor, limit: 20 },
        throwOnError: true,
      });
      this._activity = [...this._activity, ...(data?.items ?? [])];
      this._activityHasMore = data?.has_more ?? false;
      this._activityCursor = data?.next_cursor || undefined;
    } catch (err) {
      logError("loadMoreActivity failed:", err);
    } finally {
      this._activityLoadingMore = false;
    }
  }

  private async _addDependency(): Promise<void> {
    if (!this.task?.id || !this.project?.id || !this._depAddId) return;
    const ok = await addTaskDependency(
      this.project.id,
      this.task.id,
      this._depAddId,
    );
    this._depAddOpen = false;
    this._depAddId = "";
    this._depSearch = "";
    if (ok) {
      await this._loadDependencies();
      void this._loadActivity();
    } else {
      showToast(msg("Failed to add dependency"), { variant: "error" });
    }
  }

  /** Filter dependency candidates by the search string (title substring). */
  private _depFilteredCandidates(
    candidates: DtoTaskResponse[],
  ): DtoTaskResponse[] {
    const q = this._depSearch.trim().toLowerCase();
    if (!q) return candidates;
    return candidates.filter((c) => (c.title ?? "").toLowerCase().includes(q));
  }

  private async _removeDependency(blocksTaskId: string): Promise<void> {
    if (!this.task?.id || !this.project?.id) return;
    const ok = await removeTaskDependency(
      this.project.id,
      this.task.id,
      blocksTaskId,
    );
    if (ok) {
      await this._loadDependencies();
      void this._loadActivity();
    } else {
      showToast(msg("Failed to add/remove dependency"), { variant: "error" });
    }
  }

  // Close
  private _close() {
    this._menuOpen = false;
    this.dispatchEvent(
      new CustomEvent("close", { bubbles: true, composed: true }),
    );
  }

  // Render
  protected render() {
    const task = this.task;
    if (!task) {
      return html`
      `;
    }

    const hasCycles = (this.project?.cycle_duration ?? 0) > 0;
    const statusMap = new Map(this.statuses.map((s) => [s.id, s]));
    const createdDate = task.created_at
      ? timeAgo(new Date(task.created_at))
      : "";

    const statusOptions = this.statuses.map((s) => ({
      value: s.id ?? "",
      label: s.name ?? "",
      color: s.color,
    }));

    const memberOptions = this._members.map((m) => ({
      value: m.id ?? "",
      label: m.name ?? "Unknown",
      avatarUrl: m.avatar_url,
    }));

    const cycleOptions = [
      { value: "", label: msg("No cycle") },
      ...this._cycles.map((c) => ({ value: c.id ?? "", label: c.name ?? "" })),
    ];

    return html`
      <plume-dialog
        .open="${true}"
        size="full"
        noHeader
        noFooter
        style="--dialog-body-padding: 0;"
        @close="${this._close}"
      >
        <div class="tdd-layout">
          ${this._renderHeader(task, createdDate)}
          ${this._renderBreadcrumb(task)}
          <div class="tdd-content">
            <div class="tdd-main">
              <div class="tdd-tabs-wrap">
                <plume-tabs
                  .tabs="${[
                    { id: "description", label: msg("Description") },
                    { id: "subtasks", label: msg("Subtasks") },
                    { id: "dependencies", label: msg("Dependencies") },
                    { id: "time", label: msg("Time") },
                    { id: "attachments", label: msg("Files") },
                    { id: "activity", label: msg("Activity") },
                    { id: "comments", label: msg("Comments") },
                  ]}"
                  .value="${this._tab}"
                  @change="${(e: CustomEvent) => {
                    this._tab = e.detail;
                    if (this._tab === "activity") void this._loadActivity();
                  }}"
                ></plume-tabs>
              </div>
              <div class="tdd-tab-body content-enter" role="tabpanel" aria-labelledby="tab-${this
                ._tab}">
                ${this._tab === "description"
                  ? this._renderDescription(task)
                  : this._tab === "subtasks"
                  ? this._renderSubtasks(task)
                  : this._tab === "dependencies"
                  ? this._renderDependencies(task)
                  : this._tab === "time"
                  ? this._renderTime()
                  : this._tab === "attachments"
                  ? this._renderAttachments()
                  : this._tab === "activity"
                  ? this._renderActivity()
                  : this._tab === "comments"
                  ? this._renderComments()
                  : nothing}
              </div>
            </div>
            <div class="tdd-sidebar">
              <div class="tdd-sidebar-title">Properties</div>
              ${this._renderSidebar(
                task,
                statusOptions,
                memberOptions,
                cycleOptions,
                hasCycles,
                statusMap,
              )}
            </div>
          </div>
          ${this._confirmDelete ? this._renderDeleteConfirm() : nothing}
          ${this._showMoveModal ? this._renderMoveModal() : nothing}
          ${this._confirmDeleteSubtaskId
            ? this._renderSubtaskDeleteConfirm()
            : nothing}
        </div>
      </plume-dialog>
    `;
  }

  private _renderHeader(task: DtoTaskResponse, createdDate: string) {
    return html`
      <div class="tdd-header">
        <div class="tdd-title-row">
          ${this._editingTitle
            ? html`
              <plume-input
                class="tdd-flex-1"
                .value="${this._titleDraft}"
                ?autofocus="${true}"
                @input="${(
                  e: Event,
                ) => (this._titleDraft = (e.target as HTMLInputElement).value)}"
                @keydown="${(e: KeyboardEvent) => {
                  if (e.key === "Enter") this._saveTitle();
                  if (e.key === "Escape") this._cancelTitle();
                  e.stopPropagation();
                }}"
                @blur="${this._saveTitle}"
              ></plume-input>
            `
            : html`
              <h2
                class="tdd-title"
                @click="${this._startTitleEdit}"
              >
                ${task.title}
              </h2>
            `}
          <div class="tdd-actions">
            <button
              class="tdd-actions-btn"
              type="button"
              aria-label=${msg("Task actions")}
              @click="${() => (this._menuOpen = !this._menuOpen)}"
            >
              <plume-icon name="more-horizontal" size="14"></plume-icon>
            </button>
            ${this._menuOpen
              ? html`
                <div class="tdd-dropdown">
                  <button
                    class="tdd-dropdown-item"
                    type="button"
                    @click="${this._close}"
                  >
                    <plume-icon name="x" size="14"></plume-icon>
                    Close
                    <span class="tdd-shortcut">Esc</span>
                  </button>
                  <div class="tdd-dropdown-divider"></div>
                  <button
                    class="tdd-dropdown-item"
                    type="button"
                    @click="${() => this._duplicateTask(false)}"
                  >
                    <plume-icon name="copy" size="14"></plume-icon>
                    Duplicate task
                  </button>
                  ${this.task && (this.task.subtask_count ?? 0) > 0
                    ? html`
                      <button
                        class="tdd-dropdown-item"
                        type="button"
                        @click="${() => this._duplicateTask(true)}"
                      >
                        <plume-icon name="copy" size="14"></plume-icon>
                        Duplicate with subtasks
                      </button>
                    `
                    : nothing}
                  ${this.task && this.task.parent_task_id
                    ? html`
                      <button
                        class="tdd-dropdown-item"
                        type="button"
                        @click="${() => this._promoteToTopLevel()}"
                      >
                        <plume-icon name="corner-up-left" size="14"></plume-icon>
                        Promote to top-level task
                      </button>
                    `
                    : nothing}
                  <button
                    class="tdd-dropdown-item"
                    type="button"
                    @click="${() => this._openMoveModal()}"
                  >
                    <plume-icon name="arrow-right-left" size="14"></plume-icon>
                    Move to project…
                  </button>
                  <button
                    class="tdd-dropdown-item"
                    type="button"
                    @click="${this.#copyAsMarkdown}"
                  >
                    <plume-icon name="file-text" size="14"></plume-icon>
                    Copy as Markdown
                  </button>
                  <div class="tdd-dropdown-divider"></div>
                  <button
                    class="tdd-dropdown-item danger"
                    type="button"
                    @click="${this._promptDelete}"
                  >
                    <plume-icon name="trash-2" size="14"></plume-icon>
                    Delete task
                  </button>
                </div>
              `
              : nothing}
          </div>
        </div>
        <p class="tdd-meta">${task.id?.slice(0, 8)} · Created ${createdDate}</p>
      </div>
    `;
  }

  private _renderDescription(task: DtoTaskResponse) {
    if (this._editingDesc) {
      return html`
        <div class="tdd-desc-edit">
          <plume-task-editor
            .value=${this._descDraft}
            ?autofocus=${true}
            placeholder=${msg("Add description…")}
            @plume-change=${this._onDescChange}
            @plume-escape=${this._cancelDesc}
            @plume-save=${this._saveDesc}
          ></plume-task-editor>
        </div>
      `;
    }

    if (task.description) {
      const resolver = task.mentions ? buildResolver(task.mentions) : null;
      return html`
        <div
          class="tdd-desc-view tdd-desc-md"
          @click="${this._startDescEdit}"
        >
          <plume-task-editor
            .value=${task.description}
            .editable=${false}
            .mentionResolver=${resolver
              ? (type: string, id: string) => resolveLabel(resolver, type, id)
              : undefined}
          ></plume-task-editor>
          <span class="tdd-desc-edit-badge" title=${msg("Edit description")}>
            <plume-icon name="pencil" size="13"></plume-icon>
          </span>
        </div>
      `;
    }

    return html`
      <div class="tdd-desc-placeholder" @click="${this._startDescEdit}">
        Add description…
      </div>
    `;
  }

  // Breadcrumb navigation
  private _renderBreadcrumb(task: DtoTaskResponse) {
    const hasParent = !!task.parent_task_id;
    const hasStack = this._taskStack.length > 0;
    if (!hasParent && !hasStack) return nothing;

    return html`
      <div class="tdd-breadcrumb">
        ${hasStack
          ? html`
            <button
              class="tdd-breadcrumb-back"
              type="button"
              aria-label=${msg("Go back")}
              @click="${this._navigateBack}"
            >
              <plume-icon name="arrow-left" size="12"></plume-icon>
            </button>
          `
          : nothing}
        ${task.parent_title
          ? html`
            <span
              class="tdd-breadcrumb-parent"
              @click="${() => this._navigateToParent(task.parent_task_id)}"
            >${task.parent_title}</span>
            <span class="tdd-breadcrumb-sep">›</span>
          `
          : nothing}
        <span class="tdd-breadcrumb-current">${task.title}</span>
      </div>
    `;
  }

  // Subtasks tab
  private _renderSubtasks(task: DtoTaskResponse) {
    const allChildren = this._subtasks;
    const children = this._hideCompletedSubtasks
      ? allChildren.filter((c) => !c.completed_at)
      : allChildren;
    const doneCount = task.completed_subtask_count ??
      allChildren.filter((c) => c.completed_at).length;
    const totalCount = task.subtask_count ?? allChildren.length;
    const statusMap = new Map(
      (this.statuses ?? []).map((s) => [s.id, s]),
    );
    const memberOptions = this._members.map((m) => ({
      value: m.id ?? "",
      label: m.name ?? "Unknown",
      avatarUrl: m.avatar_url,
    }));

    return html`
      <div class="tdd-subtasks">
        <div class="tdd-subtasks-head">
          <span class="tdd-subtasks-progress">
            ${totalCount > 0
              ? `${doneCount}/${totalCount} completed`
              : "No subtasks yet"}
          </span>
          <div class="tdd-subtask-controls-row">
            <label class="tdd-subtask-hide-check">
              <input
                type="checkbox"
                .checked="${this._hideCompletedSubtasks}"
                @change="${(e: Event) => {
                  this._hideCompletedSubtasks =
                    (e.target as HTMLInputElement).checked;
                }}"
              />
              Hide done
            </label>
            <button
              class="tdd-subtask-collapse-btn${this._subtasksCollapsed
                ? " collapsed"
                : ""}"
              type="button"
              @click="${() => (this._subtasksCollapsed = !this
                ._subtasksCollapsed)}"
              aria-label="${this._subtasksCollapsed
                ? "Expand"
                : "Collapse"} subtasks"
              title="${this._subtasksCollapsed ? "Show" : "Hide"} subtask list"
            >
              <plume-icon name="chevron-down" size="14"></plume-icon>
            </button>
          </div>
        </div>
        ${!this._subtasksCollapsed && children.length > 0
          ? html`
            <div class="tdd-subtask-list" ${ref(this.#subtaskListRef)}
              @dragover="${this.#onListDragOver}"
              @dragleave="${this.#onListDragLeave}"
              @drop="${this.#onListDrop}">
              ${children.map(
                (c) =>
                  html`<div class="tdd-subtask-row" data-task-id="${c.id}">
                    <button
                      class="tdd-subtask-grip"
                      type="button"
                      draggable="true"
                      @dragstart="${(e: DragEvent) =>
                    this.#onRowDragStart(e, c.id ?? "")}"
                      @dragend="${this.#onRowDragEnd}"
                      title="Drag to reorder"
                      aria-label=${msg("Drag to reorder")}
                    >
                      <plume-icon name="grip-vertical" size="14"></plume-icon>
                    </button>
                    <plume-popover>
                      <button slot="trigger" class="tdd-subtask-sel-trigger" type="button">
                        <span class="tdd-subtask-sel-dot"
                          style="background: ${
                    statusMap.get(c.status_id ?? "")?.color ??
                      "var(--muted-foreground)"
                  }"
                          title="${
                    statusMap.get(c.status_id ?? "")?.name ?? ""
                  }">
                        </span>
                        <plume-icon name="chevron-down" size="10"></plume-icon>
                      </button>
                      <div slot="content" class="pop">
                        ${
                    this.statuses.map((s) =>
                      html`
                        <button class="opt" type="button"
                          @click="${async (e: Event) => {
                            if (this.project?.id && c.id) {
                              await updateTask(this.project.id, c.id, {
                                status_id: s.id,
                              });
                              await this._loadSubtasks();
                            }
                            this._closeSelect(e);
                          }}">
                          <span class="dot" style="background:${s
                            .color}"></span>
                          <span class="name">${s.name}</span>
                          ${s.id === c.status_id
                            ? html`<plume-icon class="check" name="check" size="14"></plume-icon>`
                            : nothing}
                        </button>
                      `
                    )
                  }
                      </div>
                    </plume-popover>

                    ${
                    this._editingSubtaskId === c.id
                      ? html`
                        <input
                          class="tdd-subtask-inline-input"
                          type="text"
                          .value="${this._editingSubtaskTitle}"
                          autofocus
                          @input="${(
                            e: Event,
                          ) => (this._editingSubtaskTitle =
                            (e.target as HTMLInputElement).value)}"
                          @keydown="${(e: KeyboardEvent) => {
                            if (e.key === "Enter") {
                              e.preventDefault();
                              this._saveSubtaskTitle(c);
                            }
                            if (e.key === "Escape") {
                              this._editingSubtaskId = "";
                            }
                            e.stopPropagation();
                          }}"
                          @blur="${() => this._saveSubtaskTitle(c)}"
                          @click="${(e: Event) => e.stopPropagation()}"
                        />
                      `
                      : html`
                        <span
                          class="tdd-subtask-title${c.completed_at
                            ? " done"
                            : ""}"
                          @dblclick="${(e: Event) => {
                            e.stopPropagation();
                            this._startEditSubtaskTitle(c);
                          }}"
                          @click="${() => this._openSubtask(c)}"
                        >${c.title}</span>
                      `
                  }

                  ${html`
                    <div class="tdd-subtask-actions">
                      <div class="tdd-subtask-assignees">
                        <plume-combobox
                          class="tdd-subtask-assignee-picker"
                          .options="${memberOptions}"
                          .value="${(c.assignees ?? []).map((a) => a.id ?? "")
                            .filter(Boolean)}"
                          placeholder=${msg("Assign")}
                          @change="${async (e: CustomEvent) => {
                            if (this.project?.id && c.id) {
                              await updateTask(this.project.id, c.id, {
                                assignee_ids: e.detail,
                              });
                              await this._loadSubtasks();
                            }
                          }}"
                        ></plume-combobox>
                      </div>
                      <button class="tdd-subtask-del" type="button"
                        @click="${(e: Event) => {
                          e.stopPropagation();
                          this._confirmDeleteSubtaskId = c.id ?? "";
                        }}"
                        aria-label=${msg("Delete subtask")}>
                        <plume-icon name="trash-2" size="12"></plume-icon>
                      </button>
                    </div>
                  `}
                  </div>`,
              )}
              <div class="tdd-indicator" ${ref(
                this.#subtaskIndicatorRef,
              )} style="display:none">
                <span class="tdd-indicator-dot"></span>
                <span class="tdd-indicator-line"></span>
                <span class="tdd-indicator-dot"></span>
              </div>
            </div>
          `
          : !this._subtasksCollapsed
          ? html`<div class="tdd-subtask-empty">No subtasks yet</div>`
          : nothing}
        ${!this._subtasksCollapsed
          ? html`
            <div class="tdd-subtask-add">
              <input
                class="tdd-subtask-input"
                type="text"
                placeholder=${msg("Add a subtask...")}
                .value="${this._subtaskTitle}"
                @input="${(
                  e: Event,
                ) => (this._subtaskTitle =
                  (e.target as HTMLInputElement).value)}"
                @keydown="${(e: KeyboardEvent) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    this._createSubtask(task);
                  }
                }}"
              />
              <plume-button
                variant="outline"
                size="sm"
                ?disabled="${this._creatingSubtask ||
                  !this._subtaskTitle.trim()}"
                @click="${() => this._createSubtask(task)}"
              >
                <plume-icon name="plus" size="14"></plume-icon>
              </plume-button>
            </div>
          `
          : nothing}
      </div>
    `;
  }

  private _openSubtask(child: DtoTaskResponse): void {
    // Push current task onto stack so the back button can return.
    if (this.task?.id) {
      this._taskStack = [...this._taskStack, this.task.id];
    }
    selectTask(child.id ?? "");
    this._tab = "subtasks";
  }

  private _startEditSubtaskTitle(c: DtoTaskResponse) {
    this._editingSubtaskId = c.id ?? "";
    this._editingSubtaskTitle = c.title ?? "";
  }

  private async _saveSubtaskTitle(c: DtoTaskResponse) {
    const id = this._editingSubtaskId;
    const title = this._editingSubtaskTitle.trim();
    this._editingSubtaskId = "";
    if (!id || !title || !this.project?.id || !c.id) return;
    if (title === c.title) return;
    await updateTask(this.project.id, c.id, { title });
    await this._loadSubtasks();
  }

  // Deleting, assigning, and status changes for subtask rows
  // are handled inline in _renderSubtasks via the template handlers.
  // See the popover/combobox/confirm logic in the subtask row template above.

  /** Close the enclosing single-select popover after a choice. */
  private _closeSelect(e: Event) {
    const pop = (e.target as HTMLElement | null)?.closest("plume-popover") as
      | ({ open: boolean })
      | null;
    if (pop) pop.open = false;
  }

  /* ---- Native HTML5 DnD for subtask reorder ----
   *
   * Native drag-and-drop is reliable inside shadow DOM (events fire on the
   * elements themselves, unlike @atlaskit/pragmatic-dnd which needs an
   * unbroken light-DOM chain). The grip handle is the drag source; the
   * list container is the drop target. */

  #onRowDragStart(e: DragEvent, taskId: string): void {
    if (!taskId) return;
    this.#draggingId = taskId;
    e.dataTransfer?.setData("text/plain", taskId);
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = "move";
    }
    // Mark the source row so it dims during the drag.
    const row = (e.currentTarget as HTMLElement).closest(".tdd-subtask-row");
    row?.setAttribute("data-dragging", "");
  }

  #onRowDragEnd(e: DragEvent): void {
    this.#draggingId = null;
    const row = (e.currentTarget as HTMLElement).closest(".tdd-subtask-row");
    row?.removeAttribute("data-dragging");
    const container = this.#subtaskListRef.value;
    if (container) container.removeAttribute("data-over");
    const indicator = this.#subtaskIndicatorRef.value;
    if (indicator) indicator.style.display = "none";
  }

  #onListDragOver(e: DragEvent): void {
    if (!this.#draggingId) return;
    e.preventDefault(); // allow drop
    if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
    const container = this.#subtaskListRef.value;
    if (!container) return;
    container.setAttribute("data-over", "");

    const gap = computeGap(container, e.clientY, "task-id", this.#draggingId);
    const y = computeGapY(container, gap, "task-id", this.#draggingId);
    const indicator = this.#subtaskIndicatorRef.value;
    if (indicator) {
      indicator.style.top = `${y - 2}px`;
      indicator.style.display = "flex";
    }
  }

  #onListDragLeave(e: DragEvent): void {
    // Only clear when leaving the container itself (not bubbling between
    // child rows).
    const container = this.#subtaskListRef.value;
    if (!container) return;
    const related = e.relatedTarget as Node | null;
    if (related && container.contains(related)) return;
    container.removeAttribute("data-over");
    const indicator = this.#subtaskIndicatorRef.value;
    if (indicator) indicator.style.display = "none";
  }

  #onListDrop(e: DragEvent): void {
    if (!this.#draggingId) return;
    e.preventDefault();
    const container = this.#subtaskListRef.value;
    if (!container) return;

    const draggedId = this.#draggingId;
    this.#draggingId = null;

    // Clean up visual state
    container.removeAttribute("data-over");
    const indicator = this.#subtaskIndicatorRef.value;
    if (indicator) indicator.style.display = "none";
    const draggingEl = container.querySelector("[data-dragging]");
    if (draggingEl) draggingEl.removeAttribute("data-dragging");

    // Build the visible sibling list matching the render order
    const siblings = this._subtasks.filter((s) =>
      !this._hideCompletedSubtasks || !s.completed_at
    );
    const draggedTask = siblings.find((s) => s.id === draggedId);
    if (!draggedTask) return;

    const others = siblings.filter((s) => s.id !== draggedId);
    const currentIdx = siblings.findIndex((s) => s.id === draggedId);
    const gap = computeGap(container, e.clientY, "task-id", draggedId);
    const clamped = Math.max(0, Math.min(gap, others.length));

    if (clamped === currentIdx) return; // dropped in the same spot: no-op

    if (!this.project || !this.task) return;

    const newPos = this._subtaskPositionForGap(clamped, others);

    void reorderSubtasks(this.project.id!, this.task.id!, [
      { task_id: draggedId, position_key: newPos },
    ]);
    void this._loadSubtasks();
  }

  /** Compute a new subtask_position key for a gap index within the given
   * sibling list using shared lexorank logic. */
  private _subtaskPositionForGap(
    gapIndex: number,
    siblings: DtoTaskResponse[],
  ): string {
    const key = (s: DtoTaskResponse) => s.subtask_position ?? "";
    if (siblings.length === 0) return generateKeyBetween(null, null);
    if (gapIndex === 0) {
      return generateKeyBetween(null, key(siblings[0]) || "z");
    }
    if (gapIndex >= siblings.length) {
      return generateKeyBetween(
        key(siblings[siblings.length - 1]) || "0",
        null,
      );
    }
    const prev = key(siblings[gapIndex - 1]) || "0";
    const next = key(siblings[gapIndex]) || "z";
    if (prev >= next) return generateKeyBetween(null, null);
    return generateKeyBetween(prev, next);
  }

  // Dependencies tab
  private _renderDependencies(_task: DtoTaskResponse) {
    // Candidate blockers: any task in this project except the current one and
    // those already listed as blocking.
    const blockingIds = new Set(this._blocking.map((t) => t.id ?? ""));
    const candidates = projectDetail.value.tasks.filter(
      (t) => t.id !== this.task?.id && !blockingIds.has(t.id ?? ""),
    );

    const renderDepRow = (t: DtoTaskResponse, onRemove: (id: string) => void) =>
      html`
        <div class="tdd-dep-row">
          <span
            class="tdd-dep-dot"
            style="--status-dot-color: ${this.statuses.find((s) =>
              s.id === t.status_id
            )
              ?.color ?? "var(--muted-foreground)"}"
          ></span>
          <button class="tdd-dep-title" type="button" @click="${() =>
            selectTask(t.id ?? "")}">
            ${t.title}
          </button>
          ${t.completed_at
            ? html`
              <span
                class="tdd-dep-done"><plume-icon name="check" size="12"></plume-icon></span>
            `
            : nothing}
          <plume-button
            variant="ghost"
            size="sm"
            @click="${() => onRemove(t.id ?? "")}"
          >Remove</plume-button>
        </div>
      `;

    return html`
      <div class="tdd-deps">
        <div class="tdd-dep-section">
          <div class="tdd-dep-head">
            <span class="tdd-dep-heading">Blocked by</span>
            <plume-button
              variant="outline"
              size="sm"
              @click="${() => (this._depAddOpen = !this._depAddOpen)}"
            ><plume-icon name="plus" size="12"></plume-icon> Add</plume-button>
          </div>
          ${this._depAddOpen
            ? html`
              <div class="tdd-dep-add">
                <plume-popover close-on-select>
                  <button slot="trigger" class="tdd-dep-add-trigger" type="button">
                    <plume-icon name="search" size="14"></plume-icon>
                    <span class="tdd-dep-add-label">${this._depAddId
                      ? candidates.find((c) => c.id === this._depAddId)
                        ?.title ?? "Select a task…"
                      : "Search tasks…"}</span>
                    <plume-icon name="chevron-down" size="14"></plume-icon>
                  </button>
                  <div slot="content" class="tdd-dep-search-pop" @click="${(
                    e: Event,
                  ) => e.stopPropagation()}">
                    <div class="tdd-dep-search">
                      <plume-icon name="search" size="14"></plume-icon>
                      <input
                        type="text"
                        placeholder=${msg("Search tasks…")}
                        .value="${this._depSearch}"
                        @input="${(
                          e: Event,
                        ) => (this._depSearch =
                          (e.target as HTMLInputElement).value)}"
                      />
                    </div>
                    <div class="tdd-dep-search-list">
                      ${this._depFilteredCandidates(candidates).length === 0
                        ? html`<div class="tdd-dep-search-empty">No tasks found</div>`
                        : this._depFilteredCandidates(candidates).slice(0, 50)
                          .map(
                            (c) =>
                              html`
                                <button
                                  class="tdd-dep-search-opt${c.id ===
                                      this._depAddId
                                    ? " selected"
                                    : ""}"
                                  type="button"
                                  @click="${(e: Event) => {
                                    this._depAddId = c.id ?? "";
                                    this._closeSelect(e);
                                    void this._addDependency();
                                  }}"
                                >
                                  <span class="tdd-dep-search-dot" style="background:${this
                                    .statuses.find((s) => s.id === c.status_id)
                                    ?.color ??
                                    "var(--muted-foreground)"}"></span>
                                  <span class="tdd-dep-search-name">${c
                                    .title}</span>
                                </button>
                              `,
                          )}
                    </div>
                  </div>
                </plume-popover>
                <plume-button
                  variant="ghost"
                  size="sm"
                  @click="${() => {
                    this._depAddOpen = false;
                    this._depSearch = "";
                  }}"
                >Cancel</plume-button>
              </div>
            `
            : nothing}
          ${this._blocking.length > 0
            ? html`<div class="tdd-dep-list">
                ${
              this._blocking.map((t) =>
                renderDepRow(t, (id) => this._removeDependency(id))
              )
            }
              </div>`
            : html`<div class="tdd-dep-empty">Nothing blocks this task.</div>`}
        </div>

        <div class="tdd-dep-section">
          <span class="tdd-dep-heading">Blocking</span>
          ${this._blocked.length > 0
            ? html`<div class="tdd-dep-list">
                ${
              this._blocked.map((t) =>
                renderDepRow(t, () => this._removeDependencyReverse(t.id ?? ""))
              )
            }
              </div>`
            : html`<div class="tdd-dep-empty">This task blocks nothing.</div>`}
        </div>
      </div>
    `;
  }

  private async _removeDependencyReverse(blockedTaskId: string): Promise<void> {
    // "blockedTaskId is blocked by this task": remove that edge.
    if (!this.task?.id || !this.project?.id) return;
    const ok = await removeTaskDependency(
      this.project.id,
      blockedTaskId,
      this.task.id,
    );
    if (ok) {
      await this._loadDependencies();
      void this._loadActivity();
    } else {
      showToast(msg("Failed to add/remove dependency"), { variant: "error" });
    }
  }

  private async _createSubtask(parent: DtoTaskResponse): Promise<void> {
    const title = this._subtaskTitle.trim();
    if (!title || !this.project?.id || !parent.id) return;
    this._creatingSubtask = true;
    try {
      // Inherit assignees from the parent task
      const assigneeIds = (parent.assignees ?? [])
        .map((a) => a.id)
        .filter(Boolean) as string[];

      await createTask(this.project.id, {
        title,
        status_id: parent.status_id ?? "",
        priority: parent.priority || "none",
        parent_task_id: parent.id,
        assignee_ids: assigneeIds.length > 0 ? assigneeIds : undefined,
        // Inherit the parent's cycle so the subtask tracks the same sprint.
        cycle_id: parent.cycle_id || undefined,
      });
      this._subtaskTitle = "";
      await this._loadSubtasks();
    } catch {
      // createTask already logs; keep the input so the user can retry.
      showToast(msg("Failed to create subtask"), { variant: "error" });
    } finally {
      this._creatingSubtask = false;
    }
  }

  // Time tracking tab
  private _renderTime() {
    const entries = this._timeEntries;
    const activeEntry = entries.find((e) => !e.ended_at);
    const totalMinutes = entries
      .filter((e) => e.duration_minutes != null)
      .reduce((sum, e) => sum + (e.duration_minutes ?? 0), 0);

    const fmtDur = (minutes: number | null | undefined): string => {
      if (!minutes) return "0m";
      if (minutes < 60) return `${minutes}m`;
      const h = Math.floor(minutes / 60);
      const m = minutes % 60;
      return m > 0 ? `${h}h ${m}m` : `${h}h`;
    };

    const fmtTimer = (totalSeconds: number): string => {
      const h = Math.floor(totalSeconds / 3600);
      const m = Math.floor((totalSeconds % 3600) / 60);
      const s = totalSeconds % 60;
      const mm = String(m).padStart(2, "0");
      const ss = String(s).padStart(2, "0");
      return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
    };

    return html`
      <div class="tdd-time">
        <div class="tdd-time-header">
          <span class="tdd-time-header-label">Time tracking</span>
          <span class="tdd-time-header-total">${fmtDur(totalMinutes)}</span>
        </div>

        ${activeEntry
          ? html`
            <div class="tdd-timer-active">
              <plume-icon
                class="tdd-timer-icon tdd-timer-active-icon"
                name="clock"
                size="16"
              ></plume-icon>
              <span class="tdd-timer-display">${fmtTimer(
                this._timerElapsed,
              )}</span>
              <span class="tdd-timer-desc">${activeEntry.description ??
                ""}</span>
              <button
                class="tdd-timer-stop"
                type="button"
                @click="${this._stopTimer}"
                aria-label=${msg("Stop timer")}
              >
                <plume-icon name="square" size="14"></plume-icon>
              </button>
            </div>
          `
          : html`
            <div class="tdd-flex-col-gap-2">
              <div class="tdd-flex-between">
                <span class="tdd-time-section-label">Timer</span>
                <span class="tdd-time-section-hint">Track in real time</span>
              </div>
              <div class="tdd-timer-start">
                <input
                  class="tdd-timer-start-input"
                  type="text"
                  placeholder=${msg("What are you working on?")}
                  .value="${this._timerDesc}"
                  @input="${(
                    e: Event,
                  ) => (this._timerDesc =
                    (e.target as HTMLInputElement).value)}"
                />
                <plume-button
                  variant="outline"
                  size="icon"
                  ?disabled="${!this._timerDesc.trim()}"
                  @click="${this._startTimer}"
                >
                  <plume-icon name="play-circle" size="16"></plume-icon>
                </plume-button>
              </div>
            </div>
          `}

        <hr class="tdd-time-sep" />

        <div class="tdd-flex-col-gap-2">
          <div class="tdd-flex-between">
            <span class="tdd-time-section-label">Manual entry</span>
            <span class="tdd-time-section-hint">Log past time</span>
          </div>
          <div class="tdd-manual">
            <input
              class="tdd-manual-input"
              type="text"
              placeholder=${msg("Description")}
              .value="${this._manualDesc}"
              @input="${(
                e: Event,
              ) => (this._manualDesc = (e.target as HTMLInputElement).value)}"
            />
            <input
              class="tdd-manual-num"
              type="number"
              placeholder="h"
              min="0"
              .value="${this._manualHours}"
              @input="${(
                e: Event,
              ) => (this._manualHours = (e.target as HTMLInputElement).value)}"
            />
            <input
              class="tdd-manual-num"
              type="number"
              placeholder="m"
              min="0"
              max="59"
              .value="${this._manualMinutes}"
              @input="${(
                e: Event,
              ) => (this._manualMinutes =
                (e.target as HTMLInputElement).value)}"
            />
            <plume-button
              variant="outline"
              size="icon"
              ?disabled="${!this._manualDesc.trim() ||
                (!this._manualHours && !this._manualMinutes)}"
              @click="${this._addManualTimeEntry}"
            >
              <plume-icon name="plus" size="16"></plume-icon>
            </plume-button>
          </div>
        </div>

        ${this._timeError
          ? html`
            <p class="tdd-time-error" role="alert">${this._timeError}</p>
          `
          : nothing} ${entries.length > 0
          ? html`
            <div class="tdd-entry-list">
              ${entries.map(
                (e) =>
                  html`
                    <div class="tdd-entry">
                      <span class="tdd-entry-desc">${e.description ||
                        "No description"}</span>
                      <div class="tdd-entry-right">
                        <span class="tdd-entry-dur">
                          ${e.ended_at ? fmtDur(e.duration_minutes) : "running"}
                        </span>
                        ${e.ended_at
                          ? html`
                            <button
                              class="tdd-entry-del"
                              type="button"
                              @click="${() =>
                                this._deleteTimeEntry(e.id ?? "")}"
                              aria-label=${msg("Delete time entry")}
                            >
                              <plume-icon name="trash-2" size="12"></plume-icon>
                            </button>
                          `
                          : nothing}
                      </div>
                    </div>
                  `,
              )}
            </div>
          `
          : nothing}
      </div>
    `;
  }

  // Activity tab
  private _renderActivity() {
    if (this._activityLoading && this._activity.length === 0) {
      return html`<div class="tdd-placeholder">Loading activity…</div>`;
    }
    if (this._activityError) {
      return html`<div class="tdd-placeholder" role="alert">${this._activityError}</div>`;
    }
    if (!this._activity || this._activity.length === 0) {
      return html`<div class="tdd-placeholder">No activity yet.</div>`;
    }
    return html`
      <ul class="tdd-activity-list" role="list">
        ${this._activity.map((a) => this._renderActivityRow(a))}
      </ul>
      ${this._activityHasMore
        ? html`
          <div class="tdd-activity-more">
            <button
              type="button"
              class="tdd-load-more"
              ?disabled="${this._activityLoadingMore}"
              @click="${this._loadMoreActivity}"
            >
              ${this._activityLoadingMore
                ? "Loading…"
                : "Load earlier activity"}
            </button>
          </div>
        `
        : nothing}
    `;
  }

  private _renderActivityRow(a: DtoTaskActivityResponse) {
    const label = activityLabel(a);
    const time = a.created_at ? timeAgoShort(a.created_at) : "";
    return html`
      <li class="tdd-activity-row" role="listitem">
        <div class="tdd-activity-icon" aria-hidden="true">•</div>
        <div class="tdd-activity-body">
          <span class="tdd-activity-actor">${a.actor_name || "Someone"}</span>
          <span class="tdd-activity-label">${label}</span>
          ${a.old_value || a.new_value
            ? html`<span class="tdd-activity-values">
                ${a.old_value ? html`<s>${a.old_value}</s>` : nothing}
                ${
              a.new_value
                ? html`<span class="tdd-activity-new">→ ${a.new_value}</span>`
                : nothing
            }
              </span>`
            : nothing}
          <span class="tdd-activity-time">${time}</span>
        </div>
      </li>
    `;
  }

  // Attachments tab
  private _renderAttachments() {
    const fmtSize = (bytes: number | undefined): string => {
      if (!bytes) return "0 B";
      if (bytes < 1024) return `${bytes} B`;
      if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
      return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    };

    const fileIconName = (contentType?: string): string => {
      if (!contentType) return "file";
      if (contentType.startsWith("image/")) return "image";
      if (contentType.includes("pdf")) return "file-text";
      return "file";
    };

    return html`
      <div class="tdd-attach">
        <div class="tdd-attach-header">
          <span class="tdd-attach-header-label">Attachments</span>
          <div>
            <plume-button
              variant="ghost"
              size="sm"
              class="tdd-attach-btn"
              ?disabled="${this._uploading}"
              @click="${() => {
                this._fileInput?.click();
              }}"
            >
              <plume-icon name="paperclip" size="12"></plume-icon>
              ${this._uploading ? "Uploading..." : "Attach"}
            </plume-button>
            <input
              id="tdd-file-input"
              type="file"
              class="tdd-file-input"
              @change="${(e: Event) => {
                const file = (e.target as HTMLInputElement).files?.[0];
                if (file) this._uploadAttachment(file);
                (e.target as HTMLInputElement).value = "";
              }}"
            />
          </div>
        </div>
        ${this._attachments.length === 0
          ? html`
            <span class="tdd-attach-empty">No attachments</span>
          `
          : html`
            <div class="tdd-flex-col">
              ${this._attachments.map(
                (a) =>
                  html`
                    <div class="tdd-attach-file">
                      <span class="tdd-attach-file-icon">
                        <plume-icon
                          name="${fileIconName(a.content_type)}"
                          size="16"
                        ></plume-icon>
                      </span>
                      <div class="tdd-attach-file-info">
                        <a
                          class="tdd-attach-file-name"
                          href="/api/attachments/${a.id ?? ""}/download"
                          target="_blank"
                          rel="noopener noreferrer"
                        >
                          ${a.filename}
                        </a>
                        <span class="tdd-attach-file-meta">
                          ${fmtSize(a.size)} · ${timeAgoShort(a.created_at)}
                        </span>
                      </div>
                      <button
                        class="tdd-attach-file-del"
                        type="button"
                        @click="${() => this._deleteAttachment(a.id ?? "")}"
                        aria-label=${msg("Delete attachment")}
                      >
                        <plume-icon name="trash-2" size="12"></plume-icon>
                      </button>
                    </div>
                  `,
              )}
            </div>
          `}
      </div>
    `;
  }

  private _renderComments() {
    const pid = this.project?.id ?? "";
    const tid = this.task?.id ?? "";
    return html`
      <div class="tdd-comments">
        <plume-comment-thread
          .projectId="${pid}"
          .taskId="${tid}"
        ></plume-comment-thread>
      </div>
    `;
  }

  private _renderSidebar(
    task: DtoTaskResponse,
    statusOptions: { value: string; label: string; color?: string }[],
    memberOptions: { value: string; label: string; avatarUrl?: string }[],
    cycleOptions: { value: string; label: string }[],
    hasCycles: boolean,
    _statusMap: Map<string | undefined, DtoTaskStatusResponse>,
  ) {
    return html`
      <div class="tdd-prop">
        <span class="tdd-prop-label">Status</span>
        <div class="tdd-prop-value">
          <plume-select
            .options="${statusOptions}"
            .value="${task.status_id ?? ""}"
            @change="${(e: CustomEvent) => this._onStatusChange(e.detail)}"
          ></plume-select>
        </div>
      </div>
      <div class="tdd-prop">
        <span class="tdd-prop-label">Priority</span>
        <div class="tdd-prop-value">
          <plume-select
            .options="${getPriorities()}"
            .value="${task.priority ?? "none"}"
            @change="${(e: CustomEvent) => this._onPriorityChange(e.detail)}"
          ></plume-select>
        </div>
      </div>
      <div class="tdd-prop">
        <span class="tdd-prop-label">Assignees</span>
        <div class="tdd-prop-value">
          <plume-combobox
            .options="${memberOptions}"
            .value="${task.assignees?.map((a) => a.id ?? "").filter(Boolean) ??
              []}"
            placeholder=${msg("Assignees")}
            @change="${(e: CustomEvent) => this._onAssigneesChange(e.detail)}"
          ></plume-combobox>
        </div>
      </div>
      <div class="tdd-prop">
        <span class="tdd-prop-label">Labels</span>
        <div class="tdd-prop-value">
          <plume-label-picker
            .selected="${task.labels ?? []}"
            @change="${(e: CustomEvent) =>
              this._onLabelsChange(e.detail.labelIds)}"
          ></plume-label-picker>
        </div>
      </div>
      <div class="tdd-prop">
        <span class="tdd-prop-label">Start date</span>
        <div class="tdd-prop-value">
          <plume-date-field
            .value="${task.started_at ?? ""}"
            placeholder=${msg("Set start date")}
            @change="${(e: CustomEvent) => this._onStartDateChange(e.detail)}"
          ></plume-date-field>
        </div>
      </div>
      <div class="tdd-prop">
        <span class="tdd-prop-label">Due date</span>
        <div class="tdd-prop-value">
          <plume-date-field
            .value="${task.due_at ?? ""}"
            placeholder=${msg("Set due date")}
            @change="${(e: CustomEvent) => this._onDueDateChange(e.detail)}"
          ></plume-date-field>
        </div>
      </div>
      ${hasCycles
        ? html`
          <div class="tdd-prop">
            <span class="tdd-prop-label">Cycle</span>
            <div class="tdd-prop-value">
              <plume-select
                .options="${cycleOptions}"
                .value="${task.cycle_id ?? ""}"
                @change="${(e: CustomEvent) => this._onCycleChange(e.detail)}"
              ></plume-select>
            </div>
          </div>
        `
        : nothing}
      <div class="tdd-prop">
        <span class="tdd-prop-label">Estimate</span>
        <div class="tdd-prop-value">
          <plume-input
            type="number"
            min="0"
            placeholder=${msg("Hours")}
            .value="${task.estimate?.toString() ?? ""}"
            @input="${(
              e: Event,
            ) => this._onEstimateChange((e.target as HTMLInputElement).value)}"
          ></plume-input>
        </div>
      </div>
    `;
  }

  private _renderDeleteConfirm() {
    return html`
      <div class="tdd-confirm-overlay">
        <div class="tdd-confirm-box">
          <h3>Delete this task?</h3>
          <p>
            This action cannot be undone. The task and its data will be permanently
            removed.
          </p>
          <div class="tdd-confirm-actions">
            <plume-button variant="ghost" @click="${this._cancelDelete}">
              Cancel
            </plume-button>
            <plume-button
              variant="destructive"
              @click="${this._confirmDeleteTask}"
            >
              Delete task
            </plume-button>
          </div>
        </div>
      </div>
    `;
  }

  private _renderSubtaskDeleteConfirm() {
    const subtask = this._subtasks.find((s) =>
      s.id === this._confirmDeleteSubtaskId
    );
    const title = subtask?.title ?? "this subtask";
    return html`
      <div class="tdd-confirm-overlay" @click="${(e: Event) => {
        if (e.target === e.currentTarget) this._confirmDeleteSubtaskId = "";
      }}">
        <div class="tdd-confirm-box">
          <h3>Delete subtask?</h3>
          <p>
            \\u201c${title}\\u201d will be permanently removed. This action
            cannot be undone.
          </p>
          <div class="tdd-confirm-actions">
            <plume-button variant="ghost" @click="${() => (this
              ._confirmDeleteSubtaskId = "")}">
              Cancel
            </plume-button>
            <plume-button
              variant="destructive"
              ?disabled="${this._deletingSubtask}"
              @click="${this._confirmDeleteSubtask}"
            >
              ${this._deletingSubtask
                ? html`<plume-spinner></plume-spinner>`
                : "Delete subtask"}
            </plume-button>
          </div>
        </div>
      </div>
    `;
  }

  private _renderMoveModal() {
    const otherProjects = projectsSignal.value.projects.filter(
      (p) => p.id !== this.project?.id,
    );
    return html`
      <div class="tdd-confirm-overlay" @click="${(e: Event) => {
        if (e.target === e.currentTarget) this._showMoveModal = false;
      }}">
        <div class="tdd-confirm-box tdd-move-box">
          <h3>Move task to project</h3>
          <p class="tdd-move-hint">
            The task's cycle and parent will be cleared (they belong to the
            current project).
          </p>
          <div class="tdd-move-form">
            <plume-field label="Target project">
              <plume-select
                searchable
                .options="${otherProjects.map((p) => ({
                  value: p.id ?? "",
                  label: p.name ?? "",
                  color: p.color,
                }))}"
                .value="${this._moveProjectId}"
                @change="${(e: CustomEvent) => {
                  this._moveProjectId = e.detail as string;
                }}"
              ></plume-select>
            </plume-field>
          </div>
          <div class="tdd-confirm-actions">
            <plume-button
              variant="ghost"
              @click="${() => (this._showMoveModal = false)}"
            >
              Cancel
            </plume-button>
            <plume-button
              ?disabled="${this._moving || !this._moveProjectId}"
              @click="${() => this._confirmMove()}"
            >
              ${this._moving ? "Moving…" : "Move task"}
            </plume-button>
          </div>
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-task-detail-dialog": PlumeTaskDetailDialog;
  }
}

/** Human-readable label for a task activity action. */
function activityLabel(a: DtoTaskActivityResponse): string {
  switch (a.action) {
    case "created":
      return "created this task";
    case "updated":
      return "updated this task";
    case "status_changed":
      return "changed the status";
    case "assigned":
      return "was assigned";
    case "unassigned":
      return "was unassigned";
    case "priority_changed":
      return "changed the priority";
    case "due_date_changed":
      return "changed the due date";
    case "moved":
      return "moved this task";
    case "deleted":
      return "deleted this task";
    case "title_changed":
      return "changed the title";
    case "description_changed":
      return "changed the description";
    case "labels_changed":
      return "changed the labels";
    case "estimate_changed":
      return "changed the estimate";
    case "cycle_changed":
      return "changed the cycle";
    case "reparented":
      return "changed the parent";
    case "started_at_changed":
      return "changed the start date";
    case "moved_to_project":
      return "moved this task to another project";
    case "duplicated":
      return "duplicated this task";
    case "comment_added":
      return "commented on this task";
    case "file_attached":
      return "attached a file";
    case "file_removed":
      return "removed a file";
    case "time_logged":
      return "logged time";
    case "dependency_added":
      return "added a dependency";
    case "dependency_removed":
      return "removed a dependency";
    default:
      return a.field ? `updated ${a.field}` : "updated this task";
  }
}
