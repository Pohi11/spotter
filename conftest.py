"""Repository-wide pytest hooks."""

import pytest


def pytest_collection_modifyitems(items: list[pytest.Item]) -> None:
    """Copy each `@pytest.mark.spec(capability, requirement)` into the test's user properties.

    pytest writes user properties into the JUnit XML report (`--junitxml`), where
    tools/speccheck joins them with the OpenSpec requirements (decision record 0001).
    """
    for item in items:
        for marker in item.iter_markers(name="spec"):
            if len(marker.args) != 2 or not all(
                isinstance(a, str) and a.strip() for a in marker.args
            ):
                raise pytest.UsageError(
                    f"{item.nodeid}: use @pytest.mark.spec('<capability>', '<requirement>')"
                )
            capability, requirement = marker.args
            item.user_properties.append(("spec", f"{capability}: {requirement}"))
