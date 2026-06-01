
def test_that_passes():
    assert 1 == 1
    assert 12 == 12


def test_readme_contains_hello_world():
    with open("README.md") as f:
        content = f.read()
    assert "hello world" in content
