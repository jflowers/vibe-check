"""Relative side B of a cycle closed only through relative imports.

This package imports the fixturepy.rel_a subpackage as a member of the
bare relative base "..", which ripwire's --deps records as ".." alone;
the edge to rel_a comes from the member-import light parse.
"""

from .. import rel_a

__all__ = ["rel_a"]
