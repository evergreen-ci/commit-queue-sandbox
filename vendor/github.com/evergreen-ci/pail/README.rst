===========================================
``pail`` -- Blob Storage System Abstraction
===========================================

Overview
--------

Pail is a high-level Go interface to blob storage containers like AWS's
S3 and similar services. Pail also provides implementation backed by
local file systems, mostly used for testing.

Documentation
-------------

The core API documentation is in the `godoc
<https://godoc.org/github.com/evergreen-ci/pail/>`_.

Contribute
----------

Open tickets in the `EVG project <http://jira.mongodb.org/browse/EVG>`_, and
feel free to open pull requests here.

Development
-----------

The pail project uses a ``makefile`` to coordinate testing. Use the following
command to build the cedar binary: ::

  make build

The artifact is at ``build/pail``. The makefile provides the following
targets:

``test``
   Runs all tests, sequentially, for all packages.

``test-<package>``
   Runs all tests for a specific package

``RACE_DETECTOR=1 make test-package``
   As with their ``test`` counterpart, these targets run tests with
   the race detector enabled.

``lint``, ``lint-<package>``
   Installs and runs the ``gometaliter`` with appropriate settings to
   lint the project.
