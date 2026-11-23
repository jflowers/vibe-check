"""Relative side A of a cycle closed only through relative imports.

This package imports fixturepy.rel_b.api through a relative base, which
ripwire's --deps records verbatim as the target "..rel_b.api";
fixturepy.rel_b imports back with `from .. import rel_a`. Runtime
importability is not required: the pair exercises static import-graph
cycle detection (openspec/changes/relative-import-edges).
"""

from ..rel_b.api import token

__all__ = ["token"]
