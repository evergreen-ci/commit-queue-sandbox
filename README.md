# commit-queue-playground

Repository here to experiment with the commit queue or whatever else.

## Testing

# Run tests with pytest.

```
$ pip install -r requirements.txt
$ pytestme please
```

## need a later commit to put a tag on

To get code coverage information, use the --cov flag

```
$ pip install -r requirements.txt
$ pytest --cov=src --cov-report=html
```

This will generate an html coverage report in `htmlcov/` directory.
This should also cause any variant that has README in its path list to run.
