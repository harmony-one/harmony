# SPEC file overview:
# https://docs.fedoraproject.org/en-US/quick-docs/creating-rpm-packages/#con_rpm-spec-file-overview
# Fedora packaging guidelines:
# https://docs.fedoraproject.org/en-US/packaging-guidelines/


Name:		harmony
Version:	2.3.4
Release:	0%{?dist}
Summary:	harmony blockchain validator node program

License:	MIT
URL:		https://harmony.one
Source0:	%{name}-%{version}.tar
BuildArch: x86_64
Packager: Leo Chen
Requires(pre): shadow-utils
Requires: bash
Requires: systemd-rpm-macros

BuildRequires:	info
Requires:	   info
BuildRoot: ~/rpmbuild/

%description
Harmony is a sharded, fast finality, low fee, PoS public blockchain.
This is the validator node program for harmony blockchain.

%global debug_package %{nil}

%prep
%setup -q

%build
echo make %{?_smp_mflags}
exit 0


%check
./harmony --version
exit

%pre
getent group harmony >/dev/null || groupadd -r harmony
getent passwd harmony >/dev/null || \
   useradd -r -g harmony -d /home/harmony -m -s /sbin/nologin \
   -c "Harmony validator node account" harmony
exit 0


%install
install -m 0755 -d ${RPM_BUILD_ROOT}/usr/local/sbin ${RPM_BUILD_ROOT}/etc/systemd/system ${RPM_BUILD_ROOT}/etc/sysctl.d
install -m 0755 harmony ${RPM_BUILD_ROOT}/usr/local/sbin/
install -m 0644 harmony.service ${RPM_BUILD_ROOT}/etc/systemd/system/
install -m 0644 harmony-sysctl.conf ${RPM_BUILD_ROOT}/etc/sysctl.d/
exit 0

%post
%systemd_post %{name}.service

%preun
%systemd_preun %{name}.service

%postun
%systemd_postun_with_restart ${name}.service

%files
/usr/local/sbin/harmony
/etc/sysctl.d/harmony-sysctl.conf
/etc/systemd/system/harmony.service

%doc
%license



%changelog
* Tue Aug 18 2020 Leo Chen <leo at harmony dot one> 2.3.4
  - init version of the harmony node program
