# Configuration

**Which development plugin.** `/techlog:welcome use <plugin>` writes `{"devPlugin": "<plugin>"}` to `.claude/techlog.json` in the project. Skills look for that file, then for `~/.claude/techlog.json`, and use `superpowers` if neither exists.

**Which skill handles which stage.** `config/mappings.yaml` in the plugin lists, for each development plugin, the skill that handles each Techlog stage and the framing text passed to it. To support another plugin, add a block with the stages `problem`, `proposal` and `decision`, then run `/techlog:welcome use <plugin>`.

**The decision rule.** The `decision` stage in `config/mappings.yaml` holds a standing rule and a `before` list of skills. `/techlog:problem` and `/techlog:proposal` hand the rule to the development plugin's design skill. Before it invokes a skill in `before` (for `superpowers`, `writing-plans`), Claude shows a draft `decision.made` event with the spec as evidence, and records it once you confirm. The rule is advice to Claude and is not enforced: it does not apply in a session that did not start from these skills, and it can be lost when the conversation is compacted.

**The decision tree.** `config/problem-tree.yaml` in the plugin holds the questions `/techlog:problem` asks. It classifies the problem first (defect, feature request, tech debt, operational issue) and then asks a few questions for that kind. The header of the file explains the node fields.

The config files are part of the plugin. Edits made inside an installed copy are replaced when the plugin updates. To keep your changes, edit the files in the repository and load it with `claude --plugin-dir ./plugin`.
