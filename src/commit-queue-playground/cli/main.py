import click


@click.group()
@click.pass_context
def cli(ctx):
    '''This is a docstring'''
    ctx.ensure_object(dict)


def foo():
    print("foo")
    print("bar")
    print("foobar")


def greeting(name):
    """Return a greeting for the given name."""
    return "Hello, {}!".format(name)


def main():
    """Entry point into commandline."""
    return cli(obj={})

def new_func():
    return "hi"
