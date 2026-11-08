import { useState, type DragEvent, type KeyboardEvent } from "react";
import type { Task, TaskStatus } from "../api/types";
import { STATUSES, priorityMeta } from "../lib/constants";
import { formatDate, isOverdue } from "../lib/format";
import { useOrgCtx } from "./OrgLayout";
import { Avatar, Badge, cx } from "./ui";

interface Props {
  tasks: Task[];
  canDrag: boolean;
  onOpen: (id: string) => void;
  onMove: (id: string, status: TaskStatus) => void;
  onAdd?: (status: TaskStatus) => void;
}

function TaskCard({ task, canDrag, onOpen }: { task: Task; canDrag: boolean; onOpen: (id: string) => void }) {
  const { nameOf } = useOrgCtx();
  const prio = priorityMeta(task.priority);
  const overdue = isOverdue(task.due_date, task.status);
  const keyOpen = (e: KeyboardEvent) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      onOpen(task.id);
    }
  };
  return (
    <div
      role="button"
      tabIndex={0}
      draggable={canDrag}
      onClick={() => onOpen(task.id)}
      onKeyDown={keyOpen}
      onDragStart={(e: DragEvent) => {
        e.dataTransfer.setData("text/plain", task.id);
        e.dataTransfer.effectAllowed = "move";
      }}
      aria-label={`${task.title}, ${prio.label} priority`}
      className={cx("rounded-lg bg-white p-3 text-left shadow-sm ring-1 ring-slate-200 transition hover:ring-indigo-300 focus-visible:outline-2 focus-visible:outline-indigo-600", canDrag && "cursor-grab active:cursor-grabbing")}
    >
      <p className="text-sm font-medium leading-snug text-slate-900">{task.title}</p>
      {task.labels.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1">
          {task.labels.slice(0, 3).map((l) => <span key={l} className="rounded bg-slate-100 px-1.5 py-0.5 text-[11px] text-slate-600">{l}</span>)}
          {task.labels.length > 3 && <span className="text-[11px] text-slate-400">+{task.labels.length - 3}</span>}
        </div>
      )}
      <div className="mt-3 flex items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge className={prio.badge}>{prio.label}</Badge>
          {task.due_date && (
            <span className={cx("text-xs", overdue ? "font-semibold text-rose-600" : "text-slate-500")}>
              {overdue && <span className="sr-only">Overdue: </span>}
              {formatDate(task.due_date)}
            </span>
          )}
        </div>
        {task.assignee_id ? <Avatar name={nameOf(task.assignee_id)} /> : <span className="text-[11px] text-slate-400">Unassigned</span>}
      </div>
    </div>
  );
}

export default function Kanban({ tasks, canDrag, onOpen, onMove, onAdd }: Props) {
  const [over, setOver] = useState<TaskStatus | null>(null);

  return (
    // `relative` matters: sr-only text is absolutely positioned and would otherwise escape this
    // scroller and widen the whole page on small screens.
    <div className="relative -mx-4 overflow-x-auto px-4 pb-4 sm:-mx-6 sm:px-6 lg:mx-0 lg:px-0">
      <div className="grid min-w-[72rem] grid-cols-6 gap-3">
        {STATUSES.map((col) => {
          const items = tasks
            .filter((t) => t.status === col.value)
            .sort((a, b) => priorityMeta(b.priority).rank - priorityMeta(a.priority).rank || b.updated_at.localeCompare(a.updated_at));
          return (
            <section
              key={col.value}
              aria-label={`${col.label}, ${items.length} tasks`}
              onDragOver={(e) => {
                if (!canDrag) return;
                e.preventDefault();
                e.dataTransfer.dropEffect = "move";
                setOver(col.value);
              }}
              onDragLeave={(e) => !e.currentTarget.contains(e.relatedTarget as Node) && setOver(null)}
              onDrop={(e) => {
                e.preventDefault();
                setOver(null);
                const id = e.dataTransfer.getData("text/plain");
                const task = tasks.find((t) => t.id === id);
                if (task && task.status !== col.value) onMove(id, col.value);
              }}
              className={cx("flex min-h-64 flex-col rounded-xl border-t-4 bg-slate-100/70 p-2.5", col.column, over === col.value && "bg-indigo-50 ring-2 ring-indigo-300")}
            >
              <header className="mb-2.5 flex items-center justify-between px-1">
                <h3 className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                  {col.label}
                  <span className="rounded-full bg-white px-2 py-0.5 text-xs font-medium text-slate-500 ring-1 ring-slate-200">{items.length}</span>
                </h3>
                {onAdd && (
                  <button onClick={() => onAdd(col.value)} aria-label={`Add task to ${col.label}`} className="rounded p-1 text-slate-400 hover:bg-white hover:text-indigo-600">
                    <svg className="size-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true"><path d="M10.75 4.75a.75.75 0 00-1.5 0v4.5h-4.5a.75.75 0 000 1.5h4.5v4.5a.75.75 0 001.5 0v-4.5h4.5a.75.75 0 000-1.5h-4.5v-4.5z" /></svg>
                  </button>
                )}
              </header>
              <div className="flex flex-1 flex-col gap-2">
                {items.map((t) => <TaskCard key={t.id} task={t} canDrag={canDrag} onOpen={onOpen} />)}
                {items.length === 0 && <p className="m-auto px-2 py-6 text-center text-xs text-slate-400">{canDrag ? "Drop tasks here" : "No tasks"}</p>}
              </div>
            </section>
          );
        })}
      </div>
    </div>
  );
}
