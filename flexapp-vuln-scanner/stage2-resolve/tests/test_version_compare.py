import pytest

from flexapp_vuln.version_compare import compare_versions, version_in_range


@pytest.mark.parametrize("a,b,expected", [
    ("1.2.10", "1.2.9", 1),
    ("1.2.9", "1.2.10", -1),
    ("1.0", "1.0", 0),
    ("1.0", "1.0.0", -1),   # fewer tokens sorts lower
    ("1.0", "1.0-beta", -1),  # numeric run sorts before a trailing non-numeric run
    ("2.1.23296", "2.1.999", 1),
])
def test_compare_versions(a, b, expected):
    result = compare_versions(a, b)
    assert (result > 0) == (expected > 0)
    assert (result < 0) == (expected < 0)
    assert (result == 0) == (expected == 0)


def test_version_in_range_no_bounds_matches_everything():
    assert version_in_range("1.0") is True


def test_version_in_range_including_bounds():
    assert version_in_range("1.5", start_including="1.0", end_including="2.0") is True
    assert version_in_range("2.0", start_including="1.0", end_including="2.0") is True
    assert version_in_range("2.1", start_including="1.0", end_including="2.0") is False


def test_version_in_range_excluding_bounds():
    assert version_in_range("1.0", start_excluding="1.0") is False
    assert version_in_range("1.0.1", start_excluding="1.0") is True
    assert version_in_range("2.0", end_excluding="2.0") is False
    assert version_in_range("1.9.9", end_excluding="2.0") is True
