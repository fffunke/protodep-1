package repository

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/stormcat24/protodep/dependency"
	"github.com/stormcat24/protodep/helper"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type GitRepository interface {
	Open() (*OpenedRepository, error)
	ProtoRootDir() string
}

type GitHubRepository struct {
	protodepDir  string
	dep          dependency.ProtoDepDependency
	authProvider helper.AuthProvider
}

func NewGitRepository(protodepDir string, dep dependency.ProtoDepDependency, authProvider helper.AuthProvider) GitRepository {
	return &GitHubRepository{
		protodepDir:  protodepDir,
		dep:          dep,
		authProvider: authProvider,
	}
}

type OpenedRepository struct {
	Repository *git.Repository
	Dep        dependency.ProtoDepDependency
	Hash       string
}

func (r *GitHubRepository) Open() (*OpenedRepository, error) {

	branch := "master"
	if r.dep.Branch != "" {
		branch = r.dep.Branch
	}

	revision := r.dep.Revision

	reponame := r.dep.Repository()
	repopath := filepath.Join(r.protodepDir, reponame)

	var rep *git.Repository

	fmt.Println(repopath)
	if stat, err := os.Stat(repopath); err == nil && stat.IsDir() {
		rep, err = git.PlainOpen(repopath)
		if err != nil {
			return nil, errors.Wrap(err, "open repository is failed")
		}

		fetchOpts := &git.FetchOptions{
			Auth:     r.authProvider.AuthMethod(),
			Progress: os.Stdout,
		}

		if err := rep.Fetch(fetchOpts); err != nil {
			if err != git.NoErrAlreadyUpToDate {
				return nil, errors.Wrap(err, "fetch repository is failed")
			}
		}

	} else {
		rep, err = git.PlainClone(repopath, false, &git.CloneOptions{
			Auth:     r.authProvider.AuthMethod(),
			URL:      r.authProvider.GetRepositoryURL(reponame),
			Progress: os.Stdout,
		})
		if err != nil {
			return nil, errors.Wrap(err, "clone repository is failed")
		}

	}

	wt, err := rep.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "get worktree is failed")
	}

	if revision == "" {
		target, err := rep.Storer.Reference(plumbing.ReferenceName(fmt.Sprintf("refs/remotes/origin/%s", branch)))
		if err != nil {
			return nil, errors.Wrapf(err, "change branch to %s is failed", branch)
		}

		if err := wt.Checkout(&git.CheckoutOptions{Hash: target.Hash()}); err != nil {
			return nil, errors.Wrapf(err, "checkout to %s is failed", revision)
		}

		head := plumbing.NewHashReference(plumbing.HEAD, target.Hash())
		if err := rep.Storer.SetReference(head); err != nil {
			return nil, errors.Wrapf(err, "set head to %s is failed", branch)
		}
	} else  {
		hash := plumbing.NewHash(revision)
		if err := wt.Checkout(&git.CheckoutOptions{Hash: hash}); err != nil {
			return nil, errors.Wrapf(err, "checkout to %s is failed", revision)
		}

		head := plumbing.NewHashReference(plumbing.HEAD, hash)
		if err := rep.Storer.SetReference(head); err != nil {
			return nil, errors.Wrapf(err, "set head to %s is failed", revision)
		}
	}

	commiter, err := rep.Log(&git.LogOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "get commit is failed")
	}

	current, err := commiter.Next()
	if err != nil {
		return nil, errors.Wrap(err, "get commit current is failed")
	}

	return &OpenedRepository{
		Repository: rep,
		Dep: r.dep,
		Hash: current.Hash.String(),
	}, nil
}

func (r *GitHubRepository) ProtoRootDir() string {
	return filepath.Join(r.protodepDir, r.dep.Target)
}