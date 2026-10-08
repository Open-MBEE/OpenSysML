package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;
import org.openmbee.opensysml.proto.RenderViewResponse;

/** Machine-readable rendering of a named or targeted pseudo-view. */
public record RenderedView(
    String view,
    String kind,
    String stated,
    List<Node> nodes,
    List<Edge> edges,
    List<String> columns,
    List<Row> rows,
    Optional<Canvas> canvas,
    List<Note> notes,
    List<String> notices) {

  public RenderedView {
    Objects.requireNonNull(view, "view");
    Objects.requireNonNull(kind, "kind");
    Objects.requireNonNull(stated, "stated");
    nodes = List.copyOf(nodes);
    edges = List.copyOf(edges);
    columns = List.copyOf(columns);
    rows = List.copyOf(rows);
    Objects.requireNonNull(canvas, "canvas");
    notes = List.copyOf(notes);
    notices = List.copyOf(notices);
  }

  /** A flattened rendered node; parents precede children. */
  public record Node(
      String id,
      String kind,
      String name,
      boolean nameSynthesized,
      String type,
      String detail,
      String text,
      boolean standIn,
      String parent,
      List<Port> ports,
      Optional<Span> origin,
      Optional<Geometry> geometry,
      Optional<Style> style) {
    public Node {
      ports = List.copyOf(ports);
      Objects.requireNonNull(origin, "origin");
      Objects.requireNonNull(geometry, "geometry");
      Objects.requireNonNull(style, "style");
    }
  }

  /** One rendered feature on a node's boundary. */
  public record Port(String id, String name, String type, String direction) {}

  /** Position and size of a node. */
  public record Geometry(
      double x, double y, double width, double height, boolean hasSize, boolean collapsed) {}

  /** Visual attributes for a node or edge. */
  public record Style(
      String fill,
      String line,
      String text,
      String font,
      double fontSize,
      boolean bold,
      boolean italic) {}

  /** One point on an edge route. */
  public record Point(double x, double y) {}

  /** A rendered connection between nodes. */
  public record Edge(
      String from,
      String to,
      String fromPort,
      String toPort,
      String label,
      String name,
      String kind,
      Optional<Span> origin,
      List<Point> route,
      Optional<Style> style) {
    public Edge {
      Objects.requireNonNull(origin, "origin");
      route = List.copyOf(route);
      Objects.requireNonNull(style, "style");
    }
  }

  /** The drawing surface, when stated by the view. */
  public record Canvas(String unit, double width, double height, boolean hasSize) {}

  /** One table row. */
  public record Row(List<String> cells, Optional<Span> origin) {
    public Row {
      cells = List.copyOf(cells);
      Objects.requireNonNull(origin, "origin");
    }
  }

  /** A rendered note and its optional node or edge anchor. */
  public record Note(
      String text,
      String anchor,
      String edgeFrom,
      String edgeTo,
      double x,
      double y,
      double width,
      double height,
      boolean hasSize,
      Optional<Span> origin) {
    public Note {
      Objects.requireNonNull(origin, "origin");
    }
  }

  /** A source span. */
  public record Span(String file, int startLine, int startCol, int endLine, int endCol) {}

  /** Decodes the service response without discarding presence or field values. */
  public static RenderedView from(RenderViewResponse response) {
    return new RenderedView(
        response.getView(),
        response.getKind(),
        response.getStated(),
        response.getNodesList().stream()
            .map(
                node ->
                    new Node(
                        node.getId(),
                        node.getKind(),
                        node.getName(),
                        node.getNameSynthesized(),
                        node.getType(),
                        node.getDetail(),
                        node.getText(),
                        node.getStandIn(),
                        node.getParent(),
                        node.getPortsList().stream()
                            .map(
                                port ->
                                    new Port(
                                        port.getId(),
                                        port.getName(),
                                        port.getType(),
                                        port.getDirection()))
                            .toList(),
                        node.hasOrigin() ? Optional.of(span(node.getOrigin())) : Optional.empty(),
                        node.hasGeometry()
                            ? Optional.of(
                                new Geometry(
                                    node.getGeometry().getX(),
                                    node.getGeometry().getY(),
                                    node.getGeometry().getWidth(),
                                    node.getGeometry().getHeight(),
                                    node.getGeometry().getHasSize(),
                                    node.getGeometry().getCollapsed()))
                            : Optional.empty(),
                        node.hasStyle() ? Optional.of(style(node.getStyle())) : Optional.empty()))
            .toList(),
        response.getEdgesList().stream()
            .map(
                edge ->
                    new Edge(
                        edge.getFrom(),
                        edge.getTo(),
                        edge.getFromPort(),
                        edge.getToPort(),
                        edge.getLabel(),
                        edge.getName(),
                        edge.getKind(),
                        edge.hasOrigin() ? Optional.of(span(edge.getOrigin())) : Optional.empty(),
                        edge.getRouteList().stream()
                            .map(point -> new Point(point.getX(), point.getY()))
                            .toList(),
                        edge.hasStyle() ? Optional.of(style(edge.getStyle())) : Optional.empty()))
            .toList(),
        response.getColumnsList(),
        response.getRowsList().stream()
            .map(
                row ->
                    new Row(
                        row.getCellsList(),
                        row.hasOrigin() ? Optional.of(span(row.getOrigin())) : Optional.empty()))
            .toList(),
        response.hasCanvas()
            ? Optional.of(
                new Canvas(
                    response.getCanvas().getUnit(),
                    response.getCanvas().getWidth(),
                    response.getCanvas().getHeight(),
                    response.getCanvas().getHasSize()))
            : Optional.empty(),
        response.getNotesList().stream()
            .map(
                note ->
                    new Note(
                        note.getText(),
                        note.getAnchor(),
                        note.getEdgeFrom(),
                        note.getEdgeTo(),
                        note.getX(),
                        note.getY(),
                        note.getWidth(),
                        note.getHeight(),
                        note.getHasSize(),
                        note.hasOrigin() ? Optional.of(span(note.getOrigin())) : Optional.empty()))
            .toList(),
        response.getNoticesList());
  }

  private static Span span(org.openmbee.opensysml.proto.Span span) {
    return new Span(
        span.getFile(), span.getStartLine(), span.getStartCol(), span.getEndLine(), span.getEndCol());
  }

  private static Style style(org.openmbee.opensysml.proto.RenderStyle style) {
    return new Style(
        style.getFill(),
        style.getLine(),
        style.getText(),
        style.getFont(),
        style.getFontSize(),
        style.getBold(),
        style.getItalic());
  }
}
