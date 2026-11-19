/**
 * Publication to a standard SysML v2 API server: {@link org.openmbee.opensysml.repository.Publisher}
 * commits a model converted to {@code api-json} to one project branch through
 * {@link org.openmbee.opensysml.repository.SysmlApiClient}.
 *
 * <p>The Cameo and SysON plugins share this package. It reads and writes JSON with gson, which
 * this artifact declares optional, so a caller of this package declares gson itself.
 */
package org.openmbee.opensysml.repository;
