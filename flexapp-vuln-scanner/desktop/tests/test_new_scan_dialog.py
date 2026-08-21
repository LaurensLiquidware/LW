from pathlib import Path

from new_scan_dialog import NewScanDialog


def test_output_dir_auto_fills_from_package_name(qtbot):
    dialog = NewScanDialog(Path("/scan-out"))
    qtbot.addWidget(dialog)

    dialog.package_path_edit.setText("D:\\Share\\OBS-Studio.vhdx")

    assert dialog.output_dir == "/scan-out/OBS-Studio"


def test_output_dir_not_overwritten_after_manual_edit(qtbot):
    dialog = NewScanDialog(Path("/scan-out"))
    qtbot.addWidget(dialog)

    dialog.package_path_edit.setText("D:\\Share\\App.vhdx")
    dialog.output_dir_edit.setText("/custom/output")  # simulates the user typing by hand
    dialog._on_output_dir_edited_by_hand()
    dialog.package_path_edit.setText("D:\\Share\\DifferentApp.vhdx")

    assert dialog.output_dir == "/custom/output"


def test_advanced_panel_starts_hidden(qtbot):
    # isVisible() only reflects reality once the top-level widget is
    # actually shown - a widget can be "explicitly visible" yet still
    # report isVisible() == False while its ancestor chain isn't shown.
    dialog = NewScanDialog(Path("/scan-out"))
    qtbot.addWidget(dialog)
    dialog.show()

    assert dialog.advanced_panel.isVisible() is False


def test_advanced_toggle_shows_panel(qtbot):
    dialog = NewScanDialog(Path("/scan-out"))
    qtbot.addWidget(dialog)
    dialog.show()

    dialog.advanced_toggle.setChecked(True)

    assert dialog.advanced_panel.isVisible() is True


def test_nvd_api_key_blank_is_none(qtbot):
    dialog = NewScanDialog(Path("/scan-out"))
    qtbot.addWidget(dialog)

    assert dialog.nvd_api_key is None

    dialog.nvd_api_key_edit.setText("  ")
    assert dialog.nvd_api_key is None

    dialog.nvd_api_key_edit.setText("abc123")
    assert dialog.nvd_api_key == "abc123"


def test_package_path_and_output_dir_are_stripped(qtbot):
    dialog = NewScanDialog(Path("/scan-out"))
    qtbot.addWidget(dialog)

    dialog.package_path_edit.setText("  D:\\Share\\App.vhdx  ")
    assert dialog.package_path == "D:\\Share\\App.vhdx"
