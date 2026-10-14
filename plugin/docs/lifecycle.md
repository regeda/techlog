# The lifecycle chain

Techlog follows a piece of engineering work from a problem to its follow-up. Events are immutable and every link points back in time.

Software engineering: problem (`problem.reported`), proposal (`investigation.completed`, `proposal.recorded`), decision (`decision.made`), implementation (`change.implemented`), verification (`verification.completed`), release (`release.deployed`).

Reliability engineering: incident (`incident.opened`), mitigation (`mitigation.applied`), rollback (`rollback.executed`, `recovery.verified`), postmortem (`postmortem.published`), corrective action (`action.tracked`), which feeds back into the software engineering stages.

This plugin has a skill for each of the first two stages, problem and proposal. It also records the decision (`decision.made`) when you ask to plan, before `writing-plans`, after you confirm it. The other stages can be recorded with `/techlog:record`. The full model is at https://techlog.dadev.nl/docs/lifecycle.
