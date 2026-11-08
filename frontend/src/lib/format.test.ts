import { describe, expect, it } from "vitest";
import type { Activity } from "../api/types";
import { describeActivity, initials, isOverdue, timeAgo } from "./format";

describe("isOverdue", () => {
  it("flags past due dates that are not done", () => {
    expect(isOverdue("2020-01-01T00:00:00Z", "todo", "2026-11-08")).toBe(true);
  });
  it("never flags done tasks, missing dates, or today/future", () => {
    expect(isOverdue("2020-01-01T00:00:00Z", "done", "2026-11-08")).toBe(false);
    expect(isOverdue(null, "todo", "2026-11-08")).toBe(false);
    expect(isOverdue("2026-11-08T00:00:00Z", "todo", "2026-11-08")).toBe(false);
    expect(isOverdue("2026-11-09T00:00:00Z", "blocked", "2026-11-08")).toBe(false);
  });
});

describe("timeAgo", () => {
  const now = Date.parse("2026-11-08T12:00:00Z");
  it("uses the largest sensible unit", () => {
    expect(timeAgo("2026-11-08T11:59:50Z", now)).toBe("just now");
    expect(timeAgo("2026-11-08T11:30:00Z", now)).toBe("30m ago");
    expect(timeAgo("2026-11-08T09:00:00Z", now)).toBe("3h ago");
    expect(timeAgo("2026-11-06T12:00:00Z", now)).toBe("2d ago");
  });
});

describe("initials", () => {
  it("takes at most two letters", () => {
    expect(initials("Alice Johnson")).toBe("AJ");
    expect(initials("cher")).toBe("C");
    expect(initials("Mary Jane Watson")).toBe("MJ");
  });
});

describe("describeActivity", () => {
  const base: Activity = { id: "1", project_id: null, task_id: null, actor_id: "u", actor_name: "Ann", action: "", entity_title: "Fix bug", created_at: "" };
  it("describes status changes with human labels", () => {
    expect(describeActivity({ ...base, action: "task.status_changed", meta: { from: "in_progress", to: "done" } })).toBe(
      'moved "Fix bug" from In progress to Done',
    );
  });
  it("handles unassignment and unknown actions", () => {
    expect(describeActivity({ ...base, action: "task.assigned", meta: { assignee: null } })).toBe('unassigned "Fix bug"');
    expect(describeActivity({ ...base, action: "something.new" })).toBe("something.new");
  });
});
