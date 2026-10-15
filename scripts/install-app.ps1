param(
 [ValidateSet('calendar','contacts','mail')][string]$App='calendar',
 [Parameter(Mandatory)][string]$Programs,
 [string]$SharedData=(Join-Path $env:LOCALAPPDATA 'OneWorkspace/Shared'),
 [string]$PackageFolder=(Split-Path $PSScriptRoot -Parent),
 [switch]$NoLaunch
)
$ErrorActionPreference='Stop'
$names=@{calendar='Calendar.exe';contacts='Contacts.exe';mail='Mail.exe'}
$required=@($names[$App],'WorkspaceServices.exe','stalwart.exe','engine.lock.json')
$checks=@{}
Get-Content (Join-Path $PackageFolder 'SHA256SUMS.txt') | ForEach-Object {if($_ -match '^([a-f0-9]{64})\s+(.+)$'){$checks[$matches[2]]=$matches[1]}}
foreach($name in $required){$source=Join-Path $PackageFolder $name;if(-not $checks.ContainsKey($name) -or (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash.ToLowerInvariant() -ne $checks[$name]){throw "Package verification failed: $name"}}
New-Item -ItemType Directory -Force $Programs | Out-Null
foreach($name in $required){$source=Join-Path $PackageFolder $name;$target=Join-Path $Programs $name;if(Test-Path -LiteralPath $target){if((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant() -eq $checks[$name]){continue};if($name -eq 'stalwart.exe' -or $name -eq 'WorkspaceServices.exe'){throw 'Shared program version differs. Use explicit stopped maintenance; installing another app cannot replace it.'}};Copy-Item -LiteralPath $source -Destination $target -Force}
foreach($name in @('LICENSE','NOTICE.md','THIRD-PARTY-LICENSES.txt')){if(Test-Path -LiteralPath (Join-Path $PackageFolder $name)){Copy-Item -LiteralPath (Join-Path $PackageFolder $name) -Destination (Join-Path $Programs $name) -Force}}
if(-not $NoLaunch){$appPath=Join-Path $Programs $names[$App];Start-Process -FilePath $appPath -ArgumentList @('--shared-data',('"'+$SharedData+'"')) -WindowStyle Hidden}
Write-Output "Installed $($names[$App]). Shared backend and data were not removed or reset."
