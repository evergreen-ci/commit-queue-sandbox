import os
import sys
from importlib import import_module

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "src"))

main = import_module("commit-queue-playground.cli.main")


def test_that_passes():
    assert 1 == 1
    assert 12 == 12


def test_greeting():
    assert main.greeting("world") == "Hello, world!"
