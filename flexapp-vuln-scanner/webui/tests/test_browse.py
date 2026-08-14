from pathlib import Path

import browse


def test_list_directory_separates_dirs_and_filters_files_by_extension(tmp_path):
    (tmp_path / "subfolder").mkdir()
    (tmp_path / "package.vhdx").write_text("x")
    (tmp_path / "package.exe").write_text("x")
    (tmp_path / "notes.txt").write_text("x")

    dirs, files = browse.list_directory(tmp_path, file_extensions=browse.PACKAGE_EXTENSIONS)

    assert [d.name for d in dirs] == ["subfolder"]
    assert sorted(f.name for f in files) == ["package.exe", "package.vhdx"]


def test_list_directory_no_extensions_means_no_files(tmp_path):
    (tmp_path / "subfolder").mkdir()
    (tmp_path / "package.vhdx").write_text("x")

    dirs, files = browse.list_directory(tmp_path, file_extensions=None)

    assert [d.name for d in dirs] == ["subfolder"]
    assert files == []


def test_list_directory_nonexistent_path_returns_empty(tmp_path):
    dirs, files = browse.list_directory(tmp_path / "does-not-exist", file_extensions=None)
    assert dirs == []
    assert files == []


def test_list_drives_returns_at_least_one_existing_root():
    drives = browse.list_drives()
    assert len(drives) >= 1
    for drive in drives:
        assert Path(drive).exists()
