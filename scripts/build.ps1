param([string]$Go='go',[string]$Version='0.1.0-preview.3',[string]$EngineArchive='')
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
$expected='5f8370b259330e9eba65ed21bd59af92cfecd9597aea17594d803ec68b4bc291'
if(-not $EngineArchive -or !(Test-Path -LiteralPath $EngineArchive)){$EngineArchive=Join-Path $root 'dist/stalwart.zip';New-Item -ItemType Directory -Force "$root/dist"|Out-Null;Invoke-WebRequest 'https://github.com/stalwartlabs/stalwart/releases/download/v0.16.26/stalwart-x86_64-pc-windows-msvc.zip' -OutFile $EngineArchive}
if((Get-FileHash -LiteralPath $EngineArchive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expected){throw 'Official engine archive checksum mismatch'}
New-Item -ItemType Directory -Force (Join-Path $root 'dist') | Out-Null
Push-Location $root
try{
 npm run build --prefix web
 if($LASTEXITCODE){throw 'Frontend build failed'}
 $env:CGO_ENABLED='0'
 foreach($entry in @(@('calendar','Calendar.exe'),@('contacts','Contacts.exe'),@('services','WorkspaceServices.exe'))){& $Go build -buildvcs=false -trimpath -ldflags "-s -w -X github.com/oneworkspacestudio/calendar/internal/app.Version=$Version -X main.version=$Version" -o "dist/$($entry[1])" "./cmd/$($entry[0])";if($LASTEXITCODE){throw 'Go build failed'}}
 Add-Type -AssemblyName System.IO.Compression.FileSystem
 $archive=[IO.Compression.ZipFile]::OpenRead($EngineArchive)
 try{$entry=$archive.Entries | Where-Object {$_.Name -eq 'stalwart.exe'} | Select-Object -First 1;[IO.Compression.ZipFileExtensions]::ExtractToFile($entry,(Join-Path $root 'dist/stalwart.exe'),$true)}finally{$archive.Dispose()}
 $engineHash=(Get-FileHash dist/stalwart.exe -Algorithm SHA256).Hash.ToLowerInvariant()
 @{version='0.16.26';archiveSha256=$expected;engineSha256=$engineHash;source='https://github.com/stalwartlabs/stalwart/tree/v0.16.26'} | ConvertTo-Json | Set-Content dist/engine.lock.json
 Copy-Item LICENSE,NOTICE.md,BRIEF.md -Destination dist
 Get-ChildItem dist -File | Where-Object {$_.Name -ne 'SHA256SUMS.txt' -and $_.Extension -ne '.zip'} | ForEach-Object {((Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant())+'  '+$_.Name} | Set-Content dist/SHA256SUMS.txt
}finally{Pop-Location}
