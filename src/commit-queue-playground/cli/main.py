import click


@click.group()
@click.pass_context
def cli(ctx):
    '''This is a docstring'''
    ctx.ensure_object(dict)

#hi
def foo():
    print("foo")
    print("bar")
    print("foobar")


def main():
    """Entry point into commandline."""
    return cli(obj={})

