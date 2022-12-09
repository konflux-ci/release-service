# -*- coding: utf-8 -*-
from gitlint.rules import CommitRule, RuleViolation
import re


class ConventionalCommitsFormat(CommitRule):
    """This rule will enforce that each commit title follows the
    conventional commit prefix standards and the commit
    description starts with a lowercase character."""

    id = "UC1"
    name = "contrib-format-conventional-commits"

    def validate(self, commit):
        """Validate the given commit checking that the title has a
        proper prefix and the description starts with a lowercase."""

        regex_string = "^(chore|docs|feat|fix|refactor|style|test)(\(.*\))?: [a-z].*$"
        pattern = re.compile(regex_string)
        if not pattern.match(commit.message.title):
            return [
                RuleViolation(
                    self.id, f"Commit title doesn't follow the spec {regex_string}", line_nr=1
                )
            ]
