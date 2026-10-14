# Skills in the techlog plugin

- `/techlog:problem` reports a problem. It walks a decision tree to narrow the problem, shows a draft, and after you confirm records `problem.reported` in Techlog. Asking Claude to "create a Jira ticket", "file a ticket" or "log a bug" starts it. It does not create a Jira ticket; a Jira key you give is recorded as evidence.
- `/techlog:proposal` proposes a solution. It finds the problem in Techlog, collects context (the system's history and facts about the repo) for you to confirm, runs the design process of your development plugin, and records `proposal.recorded` linked to the problem.
- `/techlog:welcome` is this help desk. `/techlog:welcome how do I connect the MCP server?` answers a question, `/techlog:welcome use <plugin>` switches the development plugin, and `/techlog:welcome` alone shows the status.
- `/techlog:record` is the skill that writes events. The other skills call it; you rarely need to.
- When you ask to plan after a proposal, the plugin first shows a draft `decision.made` event with the spec as evidence. It records the event once you confirm, before `writing-plans` runs. Say skip to plan without recording.

The development plugin does the questioning and designing. The default is `superpowers`, where both `problem` and `proposal` run its brainstorming skill.
