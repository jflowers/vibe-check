"""Ring side A of a cycle closed only through a wide-form import.

This package imports the fixturepy.ring_b subpackage through the wide
from-import form, which ripwire's --deps records as the target
"fixturepy" alone; fixturepy.ring_b imports back from fixturepy.ring_a.
Runtime importability is not required: the pair exercises static
import-graph cycle detection (openspec/changes/wide-form-cycle-edges).
"""

from fixturepy import ring_b

__all__ = ["ring_b"]
