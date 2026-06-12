import importlib.util
import os

_MAIN_PATH = os.path.join(
    os.path.dirname(__file__), "..", "..", "..", "src", "commit-queue-playground", "cli",
    "main.py")
_spec = importlib.util.spec_from_file_location("main", _MAIN_PATH)
main = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(main)


def test_that_passes():
    assert 1 == 1
    assert 12 == 12


def test_greeting():
    assert main.greeting("World") == "Hello, World!"
