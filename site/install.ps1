$ErrorActionPreference = 'Stop'

if (-not [Environment]::Is64BitOperatingSystem) {
    throw 'low runs on 64-bit Windows'
}

$dir = if ($env:LOW_INSTALL) { $env:LOW_INSTALL } else { Join-Path $env:LOCALAPPDATA 'Programs\low' }
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$target = Join-Path $dir 'low.exe'
$download = "$target.download"

try {
    Invoke-WebRequest -UseBasicParsing -Uri 'https://github.com/simon-curtis/lowlang/releases/latest/download/low-windows-amd64.exe' -OutFile $download
    Move-Item -Force -Path $download -Destination $target
}
finally {
    Remove-Item -Force -ErrorAction SilentlyContinue -Path $download
}

$environmentKey = 'HKCU:\Environment'
$userPath = (Get-Item -Path $environmentKey).GetValue('Path', '', 'DoNotExpandEnvironmentNames')
$entries = @($userPath -split ';' | Where-Object { $_ })
if ($entries -notcontains $dir) {
    $updated = (@($dir) + $entries) -join ';'
    Set-ItemProperty -Path $environmentKey -Name 'Path' -Value $updated -Type ExpandString
    $env:Path = "$dir;$env:Path"
    Add-Type -Namespace Low -Name Settings -MemberDefinition @'
[DllImport("user32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
public static extern IntPtr SendMessageTimeout(IntPtr window, uint message, UIntPtr wParam, string lParam, uint flags, uint timeout, out UIntPtr result);
'@
    $broadcast = [IntPtr]0xffff
    $settingChange = 0x1a
    $abortIfHung = 2
    $result = [UIntPtr]::Zero
    [Low.Settings]::SendMessageTimeout($broadcast, $settingChange, [UIntPtr]::Zero, 'Environment', $abortIfHung, 5000, [ref]$result) | Out-Null
}

Write-Host "installed $target"
