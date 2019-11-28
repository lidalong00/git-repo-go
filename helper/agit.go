// Copyright © 2019 Alibaba Co. Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helper

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"code.alibaba-inc.com/force/git-repo/cap"
	"code.alibaba-inc.com/force/git-repo/config"
	"code.alibaba-inc.com/force/git-repo/encode"
	"code.alibaba-inc.com/force/git-repo/project"
	log "github.com/jiangxin/multi-log"
)

// AGitHelper implements helper for AGit server.
type AGitHelper struct {
}

// GetGitPushCommand reads JSON from reader, and format it into proper JSON
// contains git push command.
func (v AGitHelper) GetGitPushCommand(reader io.Reader) ([]byte, error) {
	var (
		gitPushCmd = GitPushCommand{}
		o          = project.UploadOptions{}
		err        error
	)

	decoder := json.NewDecoder(reader)
	err = decoder.Decode(&o)
	if err != nil {
		return nil, err
	}

	cmds := []string{"git", "push"}

	if o.ReviewURL == "" {
		return nil, fmt.Errorf("review url not configured for '%s'", o.ProjectName)
	}
	if !strings.HasSuffix(o.ReviewURL, "/") {
		o.ReviewURL += "/"
	}
	url := o.ReviewURL + o.ProjectName + ".git"

	gitURL := config.ParseGitURL(url)
	if gitURL == nil || (gitURL.Proto != "ssh" && gitURL.Proto != "http" && gitURL.Proto != "https") {
		return nil, fmt.Errorf("bad review URL: %s", url)
	}
	if gitURL.IsSSH() {
		gitPushCmd.Env = []string{"AGIT_FLOW=1"}
		// TODO: obsolete, removed later.
		cmds = append(cmds, "--receive-pack=agit-receive-pack")
	} else {
		gitPushCmd.GitConfig = []string{`http.extraHeader="AGIT-FLOW: 1"`}
	}

	gitCanPushOptions := cap.GitCanPushOptions()
	if len(o.PushOptions) > 0 {
		if !gitCanPushOptions {
			log.Warnf("cannot send push options, for your git version is too low")
		} else {
			for _, pushOption := range o.PushOptions {
				cmds = append(cmds, "-o", pushOption)
			}
		}
	}

	uploadType := ""
	refSpec := ""
	localBranch := o.LocalBranch
	if strings.HasPrefix(localBranch, config.RefsHeads) {
		localBranch = strings.TrimPrefix(localBranch, config.RefsHeads)
	}
	if localBranch == "" {
		refSpec = "HEAD"
	} else {
		refSpec = config.RefsHeads + localBranch
	}

	if o.Draft {
		uploadType = "drafts"
	} else {
		uploadType = "for"
	}

	destBranch := o.DestBranch
	if strings.HasPrefix(destBranch, config.RefsHeads) {
		destBranch = strings.TrimPrefix(destBranch, config.RefsHeads)
	}

	refSpec += fmt.Sprintf(":refs/%s/%s/%s",
		uploadType,
		destBranch,
		localBranch)

	if gitCanPushOptions {
		if o.Title != "" {
			cmds = append(cmds, "-o", "title="+encode.B64Encode(o.Title))
		}
		if o.Description != "" {
			cmds = append(cmds, "-o", "description="+encode.B64Encode(o.Description))
		}
		if o.Issue != "" {
			cmds = append(cmds, "-o", "issue="+encode.B64Encode(o.Issue))
		}
		if o.People != nil && len(o.People) > 0 && len(o.People[0]) > 0 {
			reviewers := strings.Join(o.People[0], ",")
			cmds = append(cmds, "-o", "reviewers="+encode.B64Encode(reviewers))
		}
		if o.People != nil && len(o.People) > 1 && len(o.People[1]) > 0 {
			cc := strings.Join(o.People[1], ",")
			cmds = append(cmds, "-o", "cc="+encode.B64Encode(cc))
		}

		if o.NoEmails {
			cmds = append(cmds, "-o", "notify=no")
		}
		if o.Private {
			cmds = append(cmds, "-o", "private=yes")
		}
		if o.WIP {
			cmds = append(cmds, "-o", "wip=yes")
		}
	} else {
		opts := []string{}
		if o.People != nil && len(o.People) > 0 {
			for _, u := range o.People[0] {
				opts = append(opts, "r="+u)
			}
		}
		if o.People != nil && len(o.People) > 1 {
			for _, u := range o.People[1] {
				opts = append(opts, "cc="+u)
			}
		}
		if o.NoEmails {
			opts = append(opts, "notify=NONE")
		}
		if o.Private {
			opts = append(opts, "private")
		}
		if o.WIP {
			opts = append(opts, "wip")
		}
		if len(opts) > 0 {
			refSpec = refSpec + "%" + strings.Join(opts, ",")
		}
	}

	cmds = append(cmds, url, refSpec)

	gitPushCmd.Cmd = cmds[0]
	gitPushCmd.Args = cmds[1:]
	return json.Marshal(&gitPushCmd)
}

// GetDownloadRef returns reference name of the specific code review.
func (v AGitHelper) GetDownloadRef(cr, patch string) (string, error) {
	_, err := strconv.Atoi(cr)
	if err != nil {
		return "", fmt.Errorf("bad review ID %s: %s", cr, err)
	}
	return fmt.Sprintf("refs/merge-requests/%s/head", cr), nil
}