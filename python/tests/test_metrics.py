from naos import (
    Channel,
    MetricKind,
    MetricType,
    Session,
    describe_metric,
    list_metrics,
    read_double_metrics,
    read_long_metrics,
)

from fake import FakeDeviceTransport


async def open_session():
    transport = FakeDeviceTransport()
    channel = Channel(transport, None, 1)
    session = await Session.open(channel)
    return transport, channel, session


async def test_metrics_list():
    transport, channel, session = await open_session()

    metrics = await list_metrics(session)
    assert len(metrics) == 2
    assert metrics[0].ref == 0
    assert metrics[0].kind == MetricKind.GAUGE
    assert metrics[0].type == MetricType.DOUBLE
    assert metrics[0].name == "co2"
    assert metrics[0].size == 16
    assert metrics[1].name == "uptime"
    assert metrics[1].kind == MetricKind.COUNTER
    assert metrics[1].type == MetricType.LONG

    await channel.close()


async def test_metrics_describe():
    transport, channel, session = await open_session()

    layout = await describe_metric(session, 0)
    assert layout.keys == ["room"]
    assert layout.values == [["lab", "office"]]

    layout = await describe_metric(session, 1)
    assert layout.keys == []
    assert layout.values == []

    await channel.close()


async def test_metrics_read():
    transport, channel, session = await open_session()

    assert await read_double_metrics(session, 0) == [412.5, 587.0]
    assert await read_long_metrics(session, 1) == [4711]

    await channel.close()
