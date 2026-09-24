# The weekly cycle

dkt is built around one week at a time and one weekly meeting. This page explains the model behind the Week list, leftovers and the export window.

## Week and backlog

Every open task is in one of two places:

- **This week:** the tasks you plan to work on now.
- **The backlog:** everything else.

A new task goes on the week unless you ask for the backlog. This fits the usual case: you add a task because you are about to do it. The backlog is for tasks you want to remember but have not planned.

A week starts on Monday at 00:00 local time.

## Why done tasks stay on the Week list

When you mark a task done, it stays on the Week list until the next Monday. The row does not vanish under the cursor when you tick it, and you can see what you finished this week. If you tick the wrong task, you can untick it where it is, without searching History. After Monday, it moves to History.

## Leftovers

When a task goes on the week, dkt records the date. If the task is still open on the week after the next Monday, it is a **leftover**.

A leftover is not a problem in itself. It is a question: do you still plan to do this now? dkt asks it once, at the start of the week, and gives three answers:

- **Keep.** The task stays on the week, and dkt records today's date. It stops being a leftover until next Monday.
- **Defer.** The task goes back to the backlog.
- **Drop.** The task is closed, not done.

Without this step, a Week list tends to grow into a second backlog. Tasks carry over week after week, and the list stops meaning "this week".

A dropped task is not a leftover. A task that you put on the week today is not a leftover, even if you created it long ago. The date that counts is the date it went on the week.

## Work and personal

Each task is **work** or **personal**. The Week list shows the two groups apart, so personal tasks are visible but separate.

The main reason for the split is the export. The export is for a work meeting, so it only includes work tasks. You can keep your personal tasks in the same tool without them appearing in a report.

A task in a project takes the project's category. This keeps a project's tasks together in the export. Only inbox tasks, which have no project, have their own category.

## The meeting window

The export answers the question "what happened since the last meeting?". Its default window runs from one meeting to the next, using `meeting_day` and `meeting_time` from the config.

- **Before this week's meeting,** the window runs from last week's meeting to now. You can run the export an hour before the meeting and see everything up to that moment.
- **After this week's meeting,** the window ends at the meeting. A report run on Friday still describes the week that the meeting covered. Tasks finished after the meeting go in next week's report.

dkt builds each meeting time from the date and the clock time, not by adding 7 days of hours. On a week with a daylight saving change, a 15:00 meeting still starts at 15:00.

Only the **Done** section uses the window. **In progress**, **Blocked** and **Next** show the current state, because the meeting wants to know where things are now.

## See also

- [How to prepare for your weekly meeting](../how-to/weekly-meeting.md)
- [CLI reference: `export`](../reference/cli.md#export)
