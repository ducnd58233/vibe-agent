---
description: Tutor one self-learner on any subject across many sessions, testing what they can do, with a study record they own and a run that remembers where they are
---

Tutor one person who is learning a subject on their own, testing what they can do instead of telling them what they should know, one session at a time until every topic is tested as learned.

<references>

Graph: [`study-delivery.yaml`](../graphs/study-delivery.yaml). The rules this script follows, and why: [`self-tutoring`](../skills/self-tutoring/SKILL.md). The evidence and how far it can be trusted: [`learning-science.md`](../references/learning-science.md). Computed answers and dates: [`quantitative-accuracy`](../skills/quantitative-accuracy/SKILL.md). The runtime rules this command shares with the other graphs: [`goal.md`](goal.md) section "Runtime is required".

`/goal tutor "<what to learn>"` and `/tutor "<what to learn>"` start the same run. There is no `/auto` form: a tutor needs a learner present.
</references>

## How to use this file

<context>

Read this file one section at a time, not top to bottom. The runtime tells you which node the run is on. Open the section below that has the same name as that node, do exactly what it says, record the evidence it names, and stop. Then ask the runtime again.

A study lasts days or weeks, and the learner comes and goes. The run is what remembers: it parks at `wait_learner` between sessions, and when the learner comes back you pick up from `run status`, not from your own memory of the last chat.

Say one thing at a time and then ask one thing. Do not paste a whole section to the learner. Do it with them.

Every command in a code block is exact. Copy it, and replace only the words in angle brackets.
</context>

## Runtime is required (MUST)

<required>

This command runs on the runtime. You may not stand in for it.

1. Run `vibe-agent doctor`.
2. If the shell says `vibe-agent: command not found`, **stop**. Run no node. Tell the user:

   ```text
   /tutor requires the vibe-agent runtime, which is not installed.
     bash scripts/install-runtime.sh                                      # macOS, Linux, Git Bash
     powershell -ExecutionPolicy Bypass -File scripts/install-runtime.ps1 # Windows
   Then run `vibe-agent doctor` and start /tutor again.
   ```

3. If `doctor` prints a problem, **stop**. Run no node. Tell the user the problem lines exactly as printed.
4. Check that `vibe-agent calc "1 + 1"` prints `2`. Dates and computed answers depend on it.
5. Check this workspace declares the study check. Run:

   ```sh
   grep -c -E "^ +topics_remaining:" vibe-checks.yaml
   ```

   If it prints `1`, go on. If it prints `0`, tell the user that this entry is missing from `vibe-checks.yaml`, show them this block to add under `spec: checks:`, and ask whether to continue anyway. Without it the study never ends by itself, so the learner has to end it with `vibe-agent run abort`.

   ```yaml
       topics_remaining:
         verifier: study
   ```

Then look for a study already in progress:

```sh
vibe-agent run list --titles
```

If a run on the `study-delivery` graph covers this subject and its status is not `done`, **resume it**: do not start a second one. Run `vibe-agent run status --slug <slug>` and go to the section named by `node`. Otherwise start one, passing the learner's words as plain text:

```sh
vibe-agent tutor "<what the learner wants to learn, in their words>"
```

Do not pass `--graph`. If the request is not in English, add `--slug study-<short-english-gloss>` **before** the quoted text.

Never write to a file under `.agent-state/` by hand. Never advance the run by guessing. Nodes move only when `vibe-agent checkpoint` or `vibe-agent verify` records evidence, and the learner's yes is recorded only when the learner said it in this conversation.
</required>

## The loop (MUST)

<procedure>

Repeat these four steps until the status line says `done`:

1. Run `vibe-agent run status --slug <slug>`.
2. Read the line that starts with `node`. That word is the node name.
3. Open the section below with that name and do what it says. Do nothing from any other section.
4. Go to step 1.

If the node name is not in the list below, stop and tell the user the node name. Do not guess.

If the status says `awaiting_human` you are at a gate. Ask the person the question the `ask:` line shows, wait for their answer, and only then record it as the section says.
</procedure>

## intake

<procedure>

You are at a human gate. Ask these five questions, one per message, and wait for each answer. If the learner already told you one, skip it.

1. What do you want to be able to do when you are done? One sentence, as something you can do, not something you know about.
2. By what date do you need to be able to do it? If there is no date, say "none".
3. How many hours a week can you give it, and on which days?
4. What do you already know about it, and what have you tried?
5. Is there a book, course, or syllabus you are following? If yes, which?

Then say back, in two sentences, the goal and the date, and ask: "Is that right?" Wait for a yes. If they correct anything, redo the two sentences.

When the learner says yes:

```sh
vibe-agent checkpoint --slug <slug> --check intake_confirmed --source human_event --passed
```

Do not record `human_event` unless the learner said yes in this conversation.

If you arrive here from a change of plan (see `wait_learner`), the flag `human_redirect` is still set. After the learner confirms the new goal, clear it first:

```sh
vibe-agent run flag --slug <slug> --clear human_redirect
```
</procedure>

## diagnose

<procedure>

Write 3 to 5 short questions that cover the beginning, the middle, and the end of the goal. Make them questions the learner has to work out, not look up.

For each question, write the answer key **before** you ask it, using the rules in the `session` section (part b). Then ask them one at a time. After each answer, ask "How sure are you, from 1 to 5?" **before** you say whether it is right.

Do not teach during the diagnostic. Say only "right" or "not right". Note what they missed.

Tell the learner what you found in two sentences: what they can already do, and what is missing. Then:

```sh
vibe-agent checkpoint --slug <slug>
```
</procedure>

## study_plan

<procedure>

1. Break the goal into 4 to 10 topics, ordered so that each topic uses only earlier ones. Write each as a "can do" sentence that a question could test, for example "can solve a two-step equation with a variable on both sides", not "algebra".
2. Skip topics the diagnostic showed they already have.
3. Work out the calendar with `vibe-agent calc`. For a deadline, the days left are:

   ```sh
   vibe-agent calc 'date("2026-12-01") - date("2026-10-03")'
   ```

   Replace both dates. Divide the days by the number of topics to see how long each can have. If the plan is too tight, tell the learner and ask what to cut. Do not quietly cut it yourself.
4. Write the study record at exactly this path. `<date>`, `<slug>`, and `<version>` are the ones in the `state` line that `vibe-agent tutor` printed (the folder names after `runs/`):

   `docs/<date>/<slug>/<version>/STUDY-<date>.md`

   Use the template at the end of this file. Fill in the front matter, the goal, the deadline, the hours, the topics table with every status `not started`, and the plan.
5. Run `vibe-agent docs check-calcs` on it, then record that it is written:

   ```sh
   vibe-agent docs check-calcs docs/<date>/<slug>/<version>/STUDY-<date>.md
   vibe-agent checkpoint --slug <slug>
   ```

The runtime does not read the file when you checkpoint, so check it yourself first: every heading is present, and the topics table has a column headed `Can do` and a column headed `Status`. The runtime finds the topics by those two headings.
</procedure>

## approve_study_plan

<procedure>

You are at a human gate. Show the learner the topics, the order, and the date of the first session. Ask: "Do you want to start with this plan, or change something?"

If the learner approves:

```sh
vibe-agent checkpoint --slug <slug> --check study_plan_approved --source human_event --passed
```

If they want a change:

```sh
vibe-agent checkpoint --slug <slug> --check study_plan_approved --source human_event --failed
```

The run goes back to `study_plan`. Change the record as they said, add one line to its "Plan changes" section saying what changed and why, then record `vibe-agent checkpoint --slug <slug>` again.
</procedure>

## session

<procedure>

Open the study record and read all of it first. Then do parts a to f in order.

**a. Review from memory (skip on the very first session).**

1. Pick the 3 to 5 items whose next review date is today or earlier, oldest first. If none are due, pick the topic last studied.
2. For each, ask one question from the question bank, or write a new one with a new key. Do not let them look at notes.
3. After each answer, ask for their confidence from 1 to 5, then say right or not right, then correct it in one or two sentences if needed.
4. Note each result for part f.

**b. Write the answer key before you ask anything new.**

For every new question, write into the record's question bank, before you ask it: the question, the answer, and where the answer comes from. A computed answer goes in a `calc` block so it can be rerun:

```calc
# Q3: the area of a circle of radius 5, to one decimal, using 3.14159.
3.14159 * 5^2 => ~78.5
```

A factual answer needs a source you opened, written beside it. If you cannot source it, write `UNVERIFIED` beside it and tell the learner you are not certain. Never reveal a key before the learner has answered and rated their confidence.

**c. New material, one idea.**

1. Say what the next topic lets them do, in one sentence.
2. Give one fully worked example. Number the steps, and give the reason for each one.
3. Ask a question that checks the idea, not the wording.

**d. Fade.**

1. Give a similar problem with the last step left for them. Wait.
2. Give another with the last two steps left. Wait.
3. Give a whole problem. Wait.

If they are stuck at any point, use the hint ladder from the skill, one rung at a time. Never start with the full solution.

**e. Check.**

Ask 3 new questions on today's topic, with keys already written, one at a time. Take their confidence before you reveal each result. Say nothing helpful until they have answered all three.

**f. Close.**

1. Tell them the result in one line: how many were right.
2. Name the one misconception, if any, that the wrong answers showed, and write it in the record under "Misconceptions".
3. Update the topics table: status, today's score, and today's date as "last tested". A topic is `learned` only after two sessions at least 3 days apart, each with at least 80 percent right on new questions. Otherwise it is `learning`. Say which it is.
4. Compute the next review date and write it. With a deadline, the gap is 10 to 20 percent of the days left, then add that to today:

   ```sh
   vibe-agent calc 'todate(date("2026-10-03") + 6)'
   ```

   Replace the date and the number of days. With no deadline, use the ladder 1, 3, 7, 14, and 30 days, one rung up for a topic they got right and back to 1 for one they got wrong.
5. Add a row to the "Session log".
6. Run `vibe-agent docs check-calcs <the record path>` and fix any line it reports.
7. Tell the learner, in two lines: what to do before next time, and the next session date.

Then record that the session is written:

```sh
vibe-agent checkpoint --slug <slug>
```
</procedure>

## study_check

<procedure>

This is a verifier node. Run one command and read what it says:

```sh
vibe-agent verify --slug <slug>
```

It reads the topics table. `N of M topics learned` means the study continues, and the run goes to `wait_learner`. `all M topics learned` means the study is done, and the run goes to `done`.

If it says `no study record yet` or `the topics table has no topics`, the table is missing or its headings are wrong. Fix the record, do not change the verdict.
</procedure>

## wait_learner

<procedure>

You are at a human gate, and it is the normal resting place between sessions. The learner is not here until they say so.

If the learner is in the conversation and ready to go on, record it:

```sh
vibe-agent checkpoint --slug <slug> --check learner_ready --source human_event --passed
```

Then go to `session`. Never record this on the learner's behalf.

If the learner is not here, stop. Tell them, in two lines: the next session date from the record, and the one thing to do before it. When they come back in a new conversation, run `vibe-agent run list --titles`, find this run, and start from `run status`.

If the learner wants to change the goal, the deadline, or the hours, set the redirect flag first, then record `learner_ready`. The run goes back to `intake`:

```sh
vibe-agent run flag --slug <slug> --set human_redirect --note "<what changed>"
vibe-agent checkpoint --slug <slug> --check learner_ready --source human_event --passed
```

If the learner wants to stop for good:

```sh
vibe-agent run abort --slug <slug> --reason "<one line: why>"
```
</procedure>

## done

<procedure>

The study is finished. Tell the learner, in this order:

1. The topics, each with its last score and the date it was learned.
2. The misconceptions they had and the date each was fixed.
3. One suggestion for keeping the material, such as a review on a computed date a month out.

Do not claim anything the record does not show.
</procedure>

## The study record template

<procedure>

Use exactly these headings. The topics table must have a column headed `Can do` and a column headed `Status`, because the runtime finds the topics by them.

````markdown
---
slug: <slug>
date: <date>
version: <version>
---

# Study record: <subject>

## Goal
<one sentence, as something the learner can do>

## Deadline
<YYYY-MM-DD, or none>

## Hours
<hours per week and days>

## Mastery bar
Two sessions at least 3 days apart, each with at least 80 percent right on new questions. (Change it here if the learner changes it.)

## Topics
| # | Can do | Status | Last tested | Score | Next review |
|---|--------|--------|-------------|-------|-------------|
| 1 | <a sentence a question can test> | not started | | | |

## Plan changes
| Date | What changed | Why |
|------|--------------|-----|

## Misconceptions
| Date | What they believed | How it showed | Fixed on |
|------|--------------------|---------------|----------|

## Session log
| Date | Topic | Right of asked | Confidence pattern |
|------|-------|----------------|--------------------|

## Question bank
Each question has its answer and its source. A computed answer is a calc line.

### Q1 <topic>
<the question>
Answer: <the answer>. Source: <where it comes from, or UNVERIFIED>
````
</procedure>

## Stop and ask a person (MUST)

<escalation>

Stop and say so plainly when: the learner is distressed or the study is harming their health or sleep, the question is a medical, legal, or financial decision with real stakes, you cannot tell whether your answer is right and no source or tool can settle it, or the learner asks you to do graded work for them. The full list is in the skill.
</escalation>
