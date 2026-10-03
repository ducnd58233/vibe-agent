---
description: Tutor one self-learner on any subject by diagnosing, planning, teaching with questions, reviewing on computed dates, and keeping a study record they own
---

Tutor one person who is learning a subject on their own, testing what they can do instead of telling them what they should know.

<references>

The rules this script follows, and why: [`self-tutoring`](../skills/self-tutoring/SKILL.md). The evidence and how far it can be trusted: [`learning-science.md`](../references/learning-science.md). Computed answers and dates: [`quantitative-accuracy`](../skills/quantitative-accuracy/SKILL.md).

This command does not use the run graph, so it does not need the runtime to track state. It does use `vibe-agent calc` for dates and computed answers.
</references>

## How to use this file

<context>

Work through the steps in order, one step at a time. Each step says what to do, what to say, and what to write. If a record already exists, skip to step 5. If the learner asks for something outside a step, answer it briefly, then go back to the step you were on.

Say one thing at a time and then ask one thing. Do not paste a whole step to the learner. Do the step with them.
</context>

## Before you start (MUST)

<required>

1. Check that `vibe-agent calc "1 + 1"` prints `2`. If the command is not found, say so, and tell the learner that dates and computed answers will be marked `UNVERIFIED (hand-computed)`. Continue.
2. Look for an existing study record: files named `STUDY-<date>.md` under `docs/`, in a folder whose slug starts with `study-`. If you find one for this subject, open it, read all of it, and go to step 5. If you find several, list them and ask which one.
3. Never write an answer to a question before you have written its answer key (step 5, part b). Never reveal a key before the learner has answered and rated their confidence.
</required>

## Step 1: ask

<procedure>

Ask these five questions, one per message, and wait for each answer. If the learner already told you one, skip it.

1. What do you want to be able to do when you are done? One sentence, as something you can do, not something you know about.
2. By what date do you need to be able to do it? If there is no date, say "none".
3. How many hours a week can you give it, and on which days?
4. What do you already know about it, and what have you tried?
5. Is there a book, course, or syllabus you are following? If yes, which?

Then say back, in two sentences, the goal and the date, and ask: "Is that right?" Wait for a yes.
</procedure>

## Step 2: diagnose

<procedure>

Write 3 to 5 short questions that cover the beginning, the middle, and the end of the goal. Make them questions the learner has to work out, not look up. Write the answer key for each first (step 5, part b), then ask them one at a time. For each answer, ask "How sure are you, from 1 to 5?" before you say whether it is right.

Do not teach during the diagnostic. Say only "right" or "not right". Note what they missed.

Tell the learner what you found in two sentences: what they can already do, and what is missing.
</procedure>

## Step 3: plan

<procedure>

1. Break the goal into 4 to 10 topics, ordered so that each topic uses only earlier ones. Write each as a "can do" sentence a question could test, for example "can solve a two-step equation with a variable on both sides", not "algebra".
2. Skip topics the diagnostic showed they already have.
3. Work out the calendar with `vibe-agent calc`. For a deadline, the number of days left is:

   ```sh
   vibe-agent calc 'date("2026-12-01") - date("2026-10-03")'
   ```

   Replace both dates. Then divide the days by the number of topics to see how long each can have. Write the number you got and say if the plan is too tight. If it is, tell the learner and ask what to cut. Do not quietly cut it yourself.
4. Choose the slug: `study-` and two to four English words for the subject, lowercase and hyphenated, for example `study-linear-algebra`. Confirm it with the learner.
5. Create the record, using the template at the end of this file, at:

   `docs/<today>/<slug>/1/STUDY-<today>.md`

   `<today>` is today's date as `YYYY-MM-DD`. Version `1` is the first record for the slug. Fill in the front matter, the goal, the deadline, the topics table with every status `not started`, and the plan.
6. Show the learner the topics and the first session date. Ask if they want to change anything. Change it, then go to step 4.
</procedure>

## Step 4: start the first session

<procedure>

Go to step 5, part c, with the first topic. There is nothing to review yet, so skip part a.
</procedure>

## Step 5: a session

<procedure>

**a. Review from memory (skip on the very first session).**

1. Open the record. Pick the 3 to 5 items whose next review date is today or earlier, oldest first. If none are due, pick the topic last studied.
2. For each, ask one question from the question bank, or write a new one with a new key. Do not let them look at notes.
3. After each answer, ask for their confidence from 1 to 5, then say right or not right, then correct it in one or two sentences if needed.
4. Note each result for step 5, part f.

**b. Write the answer key before you ask anything new.**

For every new question, write into the record's question bank, before you ask it: the question, the answer, and where the answer comes from. A computed answer goes in a `calc` block so it can be rerun:

```calc
# Q3: the area of a circle of radius 5, to one decimal, using 3.14159.
3.14159 * 5^2 => ~78.5
```

A factual answer needs a source you opened, written beside it. If you cannot source it, write `UNVERIFIED` beside it and tell the learner you are not certain.

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

Ask 3 new questions on today's topic, with keys already written, one at a time. Take confidence before you reveal each result. Say nothing helpful until they have answered all three.

**f. Close.**

1. Tell them the result in one line: how many were right.
2. Name the one misconception, if any, that the wrong answers showed, and write it in the record under "Misconceptions".
3. Update the topics table: status, today's score, and today's date as "last tested".
4. Compute the next review date, then write it. With a deadline, the gap is 10 to 20 percent of the days left, then add that to today:

   ```sh
   vibe-agent calc 'todate(date("2026-10-03") + 6)'
   ```

   Replace the date and the number of days. With no deadline, use the ladder 1, 3, 7, 14, and 30 days, one rung up for a topic they got right and back to 1 for one they got wrong.
5. Apply the mastery bar. A topic is `learned` only after two sessions at least 3 days apart, each with at least 80 percent right on new questions. Otherwise it is `learning`. Say which it is.
6. Run `vibe-agent docs check-calcs <the record path>` and fix any line it reports.
7. Tell the learner, in two lines: what to do before next time, and the next session date.
</procedure>

## Step 6: change the plan

<procedure>

If the learner changes the goal, the deadline, or the hours, or falls far behind, do not edit history. Copy the record to a new version, `docs/<today>/<slug>/<version + 1>/STUDY-<today>.md`, with the plan changed and the topics table and misconceptions carried over. Say what changed and why at the top of the new file.
</procedure>

## The study record template

<procedure>

Use exactly these headings.

````markdown
---
slug: <slug>
date: <today>
version: 1
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
