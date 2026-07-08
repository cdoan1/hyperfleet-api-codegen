---
skill: post-updates-to-jira
description: Review recent commits and update active Jira tickets in ROSAENG-61383 epic
---

# Post Updates to Jira

This skill reviews git commits since the last Jira update and posts progress to active tickets in the ROSAENG-61383 epic.

## Steps

1. **Identify the last Jira update time**
   - Search Jira epic ROSAENG-61383 for the most recent comment
   - Use that timestamp as the baseline for "new work"

2. **Gather commits since last update**
   - Use `git log --since="<timestamp>" --pretty=format:"%h - %s (%ar)" --no-merges`
   - Group commits by topic/component
   - Identify which Jira tickets they relate to

3. **Analyze work completed**
   - Review commit messages and changes
   - Identify:
     - Features completed
     - Bugs fixed
     - Documentation updated
     - Test coverage changes
     - New files added
   - Categorize by relevant Jira ticket

4. **Check Jira ticket status**
   - Query epic ROSAENG-61383 for linked issues
   - Use `mcp__atlassian__searchJiraIssuesUsingJql` with JQL: `"epic link" = ROSAENG-61383`
   - Identify which tickets are still active (not Done/Closed)

5. **Post updates to active tickets**
   - For each active ticket with related commits:
     - Use `mcp__atlassian__addCommentToJiraIssue` to add progress comment
     - **Keep under 8 lines total**
     - Include only:
       - Date
       - 2-4 bullet points of work done (with commit hashes)
       - Changed metrics (if any)
     - Skip verbose summaries and next steps unless critical

6. **Update epic with overall progress**
   - Post summary comment to ROSAENG-61383
   - **Maximum 10 lines**
   - Include only:
     - Date and commit count
     - 3-5 key changes (one line each)
     - Coverage/metrics if changed
     - Repository link

## Update Format - Keep It Brief

**IMPORTANT**: Humans reviewing Jira don't have time for lengthy updates. Keep updates concise and scannable.

### Rules for Brevity
- Maximum 10 lines per update
- Use bullets, not paragraphs
- State facts, skip explanations
- One sentence per item
- No verbose summaries

### For Individual Tickets

**Good** (5 lines):
```markdown
Update - <date>
- Implemented X (abc123)
- Added Y docs (def456)
- Fixed Z (ghi789)
- Coverage: 73% (+3%)
```

**Bad** (too wordy):
```markdown
## Progress Update - <date>

### Work Completed
This week we successfully implemented the X feature which allows...
[Don't do this - too long]
```

### For Epic

**Good** (8 lines):
```markdown
Update - <date>
- <number> commits since <date>
- Makefile: Added HYPERSHIFT_DIR support
- Docs: workflow.md, examples/README.md
- Examples: cluster/nodepool CRD templates
- Coverage: 73% (unchanged)
- Repo: https://github.com/org/repo
```

**Bad** (too wordy):
```markdown
## Development Update - <date range>

### Summary
We pushed <number> commits since the last update...
[Don't do this - too long]
```

### Maximum Lengths
- Individual ticket update: 5-8 lines
- Epic update: 8-10 lines
- If more detail needed: link to commit or doc

## Usage

```
/post-updates-to-jira
```

The skill will automatically:
- Find the last update timestamp
- Gather and analyze commits
- Update relevant Jira tickets
- Provide a summary of updates posted

## Requirements

- Atlassian MCP server must be configured
- Access to ROSAENG project in Jira
- Git repository must have recent commits
