"""Ring side B of a cycle closed only through a wide-form import.

This package imports from fixturepy.ring_a.api; fixturepy.ring_a closes
the cycle with `from fixturepy import ring_b`, an edge only the
member-import light parse recovers.
"""

from fixturepy.ring_a.api import marker

__all__ = ["marker"]
