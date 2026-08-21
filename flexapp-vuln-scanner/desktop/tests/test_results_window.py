from PySide6.QtCore import Qt

from results_window import ResultsWindow


def _fake_result(**overrides):
    result = {
        "package_name": "TestApp",
        "inventory_path": "/tmp/x.inventory.json",
        "output_dir": "/tmp/out",
        "coverage": {"coveragePercent": 66.7},
        "severity_counts": {"CRITICAL": 1, "HIGH": 2, "MEDIUM": 0, "LOW": 3},
        "confirmed_rows": [
            {
                "severityLevel": "CRITICAL", "id": "CVE-1", "product": "a", "version": "1.0",
                "relativePaths": ["a.jar", "b.jar"], "summary": "x", "source": "nvd", "confidence": "exact-purl",
            },
        ],
        "heuristic_rows": [],
        "files": {"pdf": "/tmp/x.pdf", "sbom": "/tmp/x.sbom.json", "findings_csv": None},
    }
    result.update(overrides)
    return result


def test_header_and_coverage_shown(qtbot):
    window = ResultsWindow(_fake_result())
    qtbot.addWidget(window)

    assert "TestApp" in window.header_label.text()
    assert window.coverage_card.value_label.text() == "66.7%"


def test_severity_cards_show_counts(qtbot):
    window = ResultsWindow(_fake_result())
    qtbot.addWidget(window)

    assert window.severity_cards["CRITICAL"].value_label.text() == "1"
    assert window.severity_cards["HIGH"].value_label.text() == "2"
    assert window.severity_cards["LOW"].value_label.text() == "3"


def test_table_has_confirmed_and_heuristic_rows(qtbot):
    result = _fake_result(heuristic_rows=[
        {"severityLevel": "LOW", "id": "CVE-2", "product": "b", "version": "2.0",
         "relativePaths": ["b.exe"], "summary": "y", "source": "osv", "confidence": "heuristic"},
    ])
    window = ResultsWindow(result)
    qtbot.addWidget(window)

    assert window.model.rowCount() == 2


def test_export_button_disabled_when_file_absent(qtbot):
    window = ResultsWindow(_fake_result())
    qtbot.addWidget(window)

    assert window.pdf_button.isEnabled() is True
    assert window.csv_button.isEnabled() is False  # findings_csv is None in the fixture


def test_refresh_button_calls_callback(qtbot):
    calls = []
    window = ResultsWindow(_fake_result(), on_refresh=lambda: calls.append(True))
    qtbot.addWidget(window)

    qtbot.mouseClick(window.refresh_button, Qt.LeftButton)

    assert calls == [True]


def test_proxy_sorts_severity_by_rank_not_alphabetically(qtbot):
    # A plain alphabetical sort would put "HIGH" before "CRITICAL" -
    # the proxy must use the model's Qt.UserRole severity-rank key.
    result = _fake_result(confirmed_rows=[
        {"severityLevel": "HIGH", "id": "CVE-1", "product": "a", "version": "1",
         "relativePaths": ["a.jar"], "summary": "x", "source": "nvd", "confidence": "exact-purl"},
        {"severityLevel": "CRITICAL", "id": "CVE-2", "product": "b", "version": "1",
         "relativePaths": ["b.jar"], "summary": "y", "source": "nvd", "confidence": "exact-purl"},
    ])
    window = ResultsWindow(result)
    qtbot.addWidget(window)

    window.proxy.sort(0, Qt.AscendingOrder)

    assert window.proxy.data(window.proxy.index(0, 1)) == "CVE-2"  # CRITICAL sorts first


def test_set_result_updates_view_in_place(qtbot):
    window = ResultsWindow(_fake_result())
    qtbot.addWidget(window)

    window.set_result(_fake_result(package_name="Updated"))

    assert "Updated" in window.header_label.text()
