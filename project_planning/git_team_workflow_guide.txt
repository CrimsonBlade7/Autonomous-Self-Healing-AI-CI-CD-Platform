GIT WORKFLOW & TEAM COLLABORATION GUIDE
========================================

Welcome to the project! This guide outlines our standard Git workflow, rules for repository collaboration, and essential commands. Follow these steps for every feature you implement to ensure a smooth, conflict-free development process.

-------------------------------------------------------------------------------
1. INDIVIDUAL STEP-BY-STEP WORKFLOW FOR IMPLEMENTING FEATURES
-------------------------------------------------------------------------------

Every time you work on a new feature, bug fix, or task, follow these exact steps in order. Never work directly on the 'main' or 'develop' branches.

Step 1: Synchronize your local repository
   Before doing anything, ensure your local machine has the latest project state.
   • Switch to the main development branch:
     git checkout main
   • Pull the latest updates from the remote repository:
     git pull origin main

Step 2: Create a new feature branch
   Create and switch to a descriptive branch dedicated to your task.
   • Command: git checkout -b feature/short-description
   • Example: git checkout -b feature/user-login-page

Step 3: Develop and commit changes incrementally
   Work on your feature. Make small, logical updates and commit them regularly.
   • Check which files you changed: git status
   • Stage the changes you want to save: git add <filename> (or git add . for all)
   • Commit with a clear, concise message: git commit -m "Add email validation to login form"

Step 4: Keep your branch updated with main
   If you have been working on your feature for a few days, your teammates might have merged their own work into 'main'. Update your branch to avoid major conflicts later.
   • While on your feature branch, run: git fetch origin
   • Merge the latest main into your feature branch: git merge origin/main
   • Resolve any merge conflicts if Git prompts you, then commit the resolution.

Step 5: Push your feature branch to the remote repository
   When your feature is complete, fully tested locally, and ready for review, push it to GitHub/GitLab/Bitbucket.
   • Command: git push origin feature/short-description

Step 6: Open a Pull Request (PR) / Merge Request (MR)
   • Go to the remote repository website.
   • Click "New Pull Request".
   • Set the "base" branch to 'main' and the "compare" branch to your feature branch.
   • Fill out the PR template, explain what you did, and assign at least one teammate to review your code.

Step 7: Address review feedback and clean up
   • Make any requested changes directly on your local feature branch, commit them, and push again. The PR will update automatically.
   • Once approved and merged by a teammate, delete your local branch to keep your environment clean:
     git checkout main
     git pull origin main
     git branch -d feature/short-description


-------------------------------------------------------------------------------
2. TEAM RULES AND GUIDELINES
-------------------------------------------------------------------------------

To maintain code quality and prevent the repository from becoming a mess, all team members must strictly follow these rules:

1. Never Commit Directly to 'main'
   The 'main' branch represents our stable production code. It must be locked, and changes should only enter via approved Pull Requests.

2. One Feature, One Branch
   Keep your branches tightly scoped. Do not fix an unrelated bug or tweak another layout inside your login-page branch. Create a separate branch for separate tasks.

3. Write Meaningful Commit Messages
   Avoid vague messages like "fixed stuff", "updates", or "asdf". 
   Use the imperative mood and specify what the commit does:
   • Good: "Fix formatting issue on registration submit button"
   • Good: "Implement JWT token storage in localStorage"
   • Bad: "worked on login"

4. Pull Frequently
   Don't work in isolation for weeks. Regularly fetch and merge 'main' into your feature branch to handle small integration issues early, rather than giant merge conflicts right before a deadline.

5. Test Locally Before Pushing
   Ensure the project builds and all tests pass locally on your machine before pushing your branch or requesting a review. Never break the build for the team.

6. Review Thoroughly
   When reviewing a teammate's Pull Request, don't just look at the code. Think about edge cases, readability, architectural consistency, and test coverage. Be constructive and polite in comments.


-------------------------------------------------------------------------------
3. COMMON & USEFUL GIT COMMANDS REFERENCE
-------------------------------------------------------------------------------

Here is a quick cheat sheet of commands you will use daily:

• Starting & Cloning
  git clone <url>               - Downloads a remote repository to your local computer.
  git init                      - Initializes a brand new local Git repository.

• Inspecting the Repository
  git status                    - Shows changed files, staged files, and current branch.
  git log                       - Shows the commit history for the current branch.
  git log --oneline             - Shows a condensed, one-line-per-commit history.
  git diff                      - Shows exact line-by-line changes made but not yet staged.

• Managing Branches
  git branch                    - Lists all local branches (* marks the current one).
  git branch -a                 - Lists all local and remote branches.
  git checkout <branch-name>    - Switches to an existing branch.
  git checkout -b <new-branch>  - Creates a new branch and immediately switches to it.
  git branch -d <branch-name>   - Deletes a local branch (use -D to force delete).

• Saving Changes
  git add <file-name>           - Stages a specific file for the next commit.
  git add .                     - Stages ALL modified and new files in the directory.
  git commit -m "message"       - Permanently saves staged changes to local history.
  git commit --amend            - Modifies your very last local commit (useful for typos).

• Sharing & Updating
  git fetch                     - Downloads history from the remote repo without merging.
  git pull                      - Fetches updates from remote and immediately merges them.
  git push origin <branch>      - Uploads your local branch commits to the remote repository.

• Undoing Things (Use with caution)
  git checkout -- <file>        - Discards local changes in a file (reverts to last commit).
  git reset HEAD <file>         - Unstages a file, keeping your actual modifications intact.
  git stash                     - Temporarily shelves uncommitted changes so you can switch branches.
  git stash pop                 - Brings back your stashed changes.
