# Contributing To Harmony

## Coding Guidelines

* In general, we follow [effective_go](https://golang.org/doc/effective_go.html)
* Code must adhere to the official [Go formatting guidelines](https://golang.org/doc/effective_go.html#formatting) (i.e. uses [goimports](https://godoc.org/golang.org/x/tools/cmd/goimports)).
* Code must be documented adhering to the official Go [commentary](https://golang.org/doc/effective_go.html#commentary) guidelines.

## Pull Request (PR)

This [github document](https://help.github.com/articles/creating-a-pull-request/) provides some guidance on how to create a pull request in github.

## PR requirement
To pursue engineering excellence, we have insisted on the highest standard for the quality of each PR.

* For each PR, please run [golint](https://github.com/golang/lint), [goimports](https://godoc.org/golang.org/x/tools/cmd/goimports), to fix the basic issues/warnings.
* Make sure you understand [How to Write a Git Commit Message](https://chris.beams.io/posts/git-commit/).
* Add a [Test] section in every PR detailing on your test process and results. If the test log is too long, please include a link to [gist](https://gist.github.com/) and add the link to the PR.

## Typical workflow example
The best practice is to reorder and squash your local commits before the PR submission to create an atomic and self-contained PR.
This [book chapter](https://git-scm.com/book/en/v2/Git-Tools-Rewriting-History) provides detailed explanation and guidance on how to rewrite the local git history.

For example, a typical workflow is like the following.
```bash
# assuming you are working on a fix of bug1, and use a local branch called "fixes_of_bug1".

git clone https://github.com/harmony-one/harmony
cd harmony

# create a local branch to keep track of the origin/master
git branch fixes_of_bug1 origin/master
git checkout fixes_of_bug_1

# make changes, build, test locally, commit changes locally
# don't forget to squash or rearrange your commits using "git rebase -i"
git rebase -i origin/master

# rebase your change on the top of the tree
git pull --rebase

# push your branch and create a PR
git push origin fixes_of_bug_1:pr_fixes_of_bug_1
```

## Licensing

Please see [our Fiduciary License Agreement](FLA.md).  By your submission of
your contribution to us, you and we mutually agree to the terms and conditions
of the agreement.
