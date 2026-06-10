
def test_that_passes():
    assert 1 == 1
    assert 12 == 12


def test_bar_baz():
    """Test that bar and baz values are distinct."""
    bar = "bar"
    baz = "baz"
    assert bar != baz
    assert bar + baz == "barbaz"
