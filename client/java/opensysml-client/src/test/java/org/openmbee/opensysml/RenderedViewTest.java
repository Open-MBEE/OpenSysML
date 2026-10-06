package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.proto.RenderCanvas;
import org.openmbee.opensysml.proto.RenderEdge;
import org.openmbee.opensysml.proto.RenderGeometry;
import org.openmbee.opensysml.proto.RenderNode;
import org.openmbee.opensysml.proto.RenderNote;
import org.openmbee.opensysml.proto.RenderPoint;
import org.openmbee.opensysml.proto.RenderPort;
import org.openmbee.opensysml.proto.RenderRow;
import org.openmbee.opensysml.proto.RenderStyle;
import org.openmbee.opensysml.proto.RenderViewResponse;
import org.openmbee.opensysml.proto.Span;

class RenderedViewTest {

  @Test
  void decodesAllFieldsAndPreservesOptionalMessagePresence() {
    RenderViewResponse response =
        RenderViewResponse.newBuilder()
            .setView("Demo::view")
            .setKind("interconnection")
            .setStated("rendered")
            .addNodes(
                RenderNode.newBuilder()
                    .setId("n0")
                    .setKind("part")
                    .setName("root")
                    .setNameSynthesized(true)
                    .setType("Demo::Part")
                    .setDetail("detail")
                    .setText("text")
                    .setStandIn(true)
                    .addPorts(
                        RenderPort.newBuilder()
                            .setId("n0.0")
                            .setName("api")
                            .setType("Demo::API")
                            .setDirection("inout"))
                    .setOrigin(Span.newBuilder().setFile("views.sysml").setStartLine(4))
                    .setGeometry(
                        RenderGeometry.newBuilder()
                            .setX(1)
                            .setY(2)
                            .setWidth(3)
                            .setHeight(4)
                            .setHasSize(true)
                            .setCollapsed(true))
                    .setStyle(
                        RenderStyle.newBuilder()
                            .setFill("#fff")
                            .setLine("#000")
                            .setText("#111")
                            .setFont("sans")
                            .setFontSize(12)
                            .setBold(true)
                            .setItalic(true)))
            .addEdges(
                RenderEdge.newBuilder()
                    .setFrom("n0")
                    .setTo("n1")
                    .setFromPort("n0.0")
                    .setToPort("n1.0")
                    .setLabel("wire")
                    .setName("wire")
                    .setKind("connection")
                    .addRoute(RenderPoint.newBuilder().setX(2).setY(3))
                    .setStyle(RenderStyle.newBuilder().setLine("#222")))
            .addColumns("a")
            .addRows(RenderRow.newBuilder().addCells("x").setOrigin(Span.newBuilder().setFile("views.sysml")))
            .setCanvas(RenderCanvas.newBuilder().setUnit("px").setWidth(800).setHeight(400).setHasSize(true))
            .addNotes(
                RenderNote.newBuilder()
                    .setText("note")
                    .setAnchor("n0")
                    .setEdgeFrom("n0")
                    .setEdgeTo("n1")
                    .setX(1)
                    .setY(2)
                    .setWidth(3)
                    .setHeight(4)
                    .setHasSize(true))
            .addNotices("notice")
            .build();

    RenderedView rendered = RenderedView.from(response);
    assertEquals("Demo::view", rendered.view());
    assertTrue(rendered.nodes().get(0).nameSynthesized());
    assertEquals("inout", rendered.nodes().get(0).ports().get(0).direction());
    assertEquals(4, rendered.nodes().get(0).origin().orElseThrow().startLine());
    assertTrue(rendered.nodes().get(0).geometry().orElseThrow().collapsed());
    assertEquals(12, rendered.nodes().get(0).style().orElseThrow().fontSize());
    assertEquals("n0.0", rendered.edges().get(0).fromPort());
    assertEquals(2, rendered.edges().get(0).route().get(0).x());
    assertEquals("x", rendered.rows().get(0).cells().get(0));
    assertTrue(rendered.canvas().orElseThrow().hasSize());
    assertEquals("n1", rendered.notes().get(0).edgeTo());
    assertEquals("notice", rendered.notices().get(0));

    RenderedView empty = RenderedView.from(RenderViewResponse.getDefaultInstance());
    assertFalse(empty.canvas().isPresent());
    assertTrue(empty.nodes().isEmpty());
  }
}
