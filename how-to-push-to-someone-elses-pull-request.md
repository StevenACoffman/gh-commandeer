# How to Push to Someone Else's Pull Request

Let's say `contributor` has submitted a pull request to your (`author`) project (`repo`). They have made changes on their
branch `feature` and have proposed to merge this into `origin/master`, where 

```shell
origin -> https://github.com/author/repo.git
```

Now say you would like to make commits to their PR and push those changes. First, add their fork as a remote called
`contributor`,

```shell
> git remote add contributor https://github.com/contributor/repo.git 
```

such that,

```shell
> git remote -v
origin      https://github.com/author/repo.git (fetch)
origin      https://github.com/author/repo.git (push)
contributor   https://github.com/contributor/repo.git  (fetch) 
contributor   https://github.com/contributor/repo.git  (push)
```

Next, pull down their list of branches,

```shell
> git fetch contributor
```

and create a new branch (`contributor-feature`) from the branch that they have created the PR from,

```shell
> git checkout -b contributor-feature contributor/feature
```

Now make any changes you need to make on this branch. If you'd like to rebase this PR on top of the master branch of
the primary repository,

```shell
> git rebase origin/master
```

Finally, push the changes back up to the PR by pushing to their branch,

```shell
git push contributor contributor-feature:feature
```

Note that if you did a rebase, you'll need to add the `--force` (or `-f`) flag after `push`. The author of the PR
also may need to explicitly allow you to push to their branch.

## Helpful Links

* [Adding commits to someone else's pull request](https://tighten.co/blog/adding-commits-to-a-pull-request)
* [Official GH Docs on pushing to a PR](https://help.github.com/en/github/collaborating-with-issues-and-pull-requests/committing-changes-to-a-pull-request-branch-created-from-a-fork)
* [How to Rebase a PR](https://github.com/edx/edx-platform/wiki/How-to-Rebase-a-Pull-Request)