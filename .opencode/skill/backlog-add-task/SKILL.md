---
name: backlog-add-task
description: Add a new task to an existing feature
---

## Purpose
Add a new task file to a feature and update the index.md task table.

## Prerequisites
- Feature directory exists in `.backlog/<feature_name>/`
- Feature acronym is known (exactly 3 letters)
- Task details are clear (title, description, implementation steps)

## Procedure

1. **Determine next task ID**
   - Find highest existing task number in feature directory
   - Increment by 1
   - Zero-pad to 3 digits
   - Format: `mol-<abc>-NNN.md`

2. **Create task file with this structure:**

   ```markdown
   # <Task Title>
   
   **ID:** mol-<abc>-NNN  
   **Status:** todo  
   **Feature:** [Feature Name](./index.md)
   
   ## Objective
   [Clear, concise statement of what this task accomplishes]
   
   ## Context
   [Why this task is needed, where it fits in the feature]
   
   ## Implementation Details
   
   ### Files to Create/Modify
   - `path/to/file.go` - [what changes]
   - `path/to/test.go` - [what tests]
   
   ### Step-by-Step Instructions
   
   1. **Step 1 title**
      - Detailed instruction
      - Code examples if helpful
      - Expected outcome
   
   2. **Step 2 title**
      - Detailed instruction
      - Expected outcome
   
   ### Code Examples
   
   ```go
   // Example implementation
   ```
   
   ## Testing
   
   - [ ] Unit tests for X
   - [ ] Integration test for Y
   - [ ] Manual verification: Z
   
   ## Acceptance Criteria
   
   - [ ] Criterion 1
   - [ ] Criterion 2
   - [ ] All tests passing
   
   ## Dependencies
   
   - Depends on: [mol-<abc>-NNN](./mol-<abc>-NNN.md) (if applicable)
   - Blocks: [mol-<abc>-NNN](./mol-<abc>-NNN.md) (if applicable)
   
   ## Notes
   
   [Any additional context, edge cases, or considerations]
   ```

3. **Update index.md task table**
   - Add new row with task ID, title, and status
   - Ensure link is correct: `[mol-<abc>-NNN](./mol-<abc>-NNN.md)`
   - Keep tasks in numerical order

4. **Verify**
   - Task file renders correctly
   - Link from index.md works
   - Task ID follows format exactly

## Example Task Addition

Adding task 4 to prompt-engine (pte):

1. Create `.backlog/prompt-engine/mol-pte-004.md`
2. Fill in template with task details
3. Add to index.md:
   ```markdown
   | [mol-pte-004](./mol-pte-004.md) | Create base Carbon template | todo | |
   ```
