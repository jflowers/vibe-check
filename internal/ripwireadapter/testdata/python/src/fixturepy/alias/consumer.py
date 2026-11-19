"""Imports a submodule under an alias."""

import fixturepy.cycle_a.api as ca


def use_alias():
    """Return the aliased submodule."""
    return ca
