"""Sibling module whose name collides with the stdlib logging module."""


def shadow():
    """Return a constant; never imported by consumer.py."""
    return 1
