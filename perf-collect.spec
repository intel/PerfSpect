# -*- mode: python ; coding: utf-8 -*-


block_cipher = None


a = Analysis(
    ['perf-collect.py'],
    pathex=[],
    datas=[('./src/libtsc.so', '.'), ('./events/bdx.txt', '.'), ('./events/clx_skx.txt', '.'), ('./events/icx.txt', '.'), ('./events/spr.txt', '.'), ('./events/srf.txt', '.')],
    hiddenimports=[],
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=['readline'],
    win_no_prefer_redirects=False,
    win_private_assemblies=False,
    cipher=block_cipher,
    noarchive=False,
)

# exclude libtinfo shared library from distributed binaries due to warning observed on Ubuntu 16.04:
#    "/bin/bash: ./_MEIuU3XMv/libtinfo.so.5: no version information available (required by /bin/bash)"
a.binaries = [bin for bin in a.binaries if not bin[0].startswith('libtinfo')]

pyz = PYZ(a.pure, a.zipped_data, cipher=block_cipher)

exe = EXE(
    pyz,
    a.scripts,
    a.binaries,
    a.zipfiles,
    a.datas,
    [],
    name='perf-collect',
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=True,
    upx_exclude=[],
    runtime_tmpdir='.',
    console=True,
    disable_windowed_traceback=False,
    argv_emulation=False,
    target_arch=None,
    codesign_identity=None,
    entitlements_file=None,
)
