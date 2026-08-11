' Silent launcher for FlexAppOneDownloadMonitor.ps1
' Runs PowerShell hidden (no console window) in STA mode, required for WinForms.
'
' If you move the .ps1 file, update the path below to match.

Set objShell = CreateObject("WScript.Shell")

scriptPath = "C:\FlexAppOneDownloadMonitor\FlexAppOneDownloadMonitor.ps1"

cmd = "powershell.exe -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -STA -File """ & scriptPath & """"

' 0 = hidden window, False = don't wait for it to exit
objShell.Run cmd, 0, False
