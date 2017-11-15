/**
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *         http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package io.syndesis.rest.v1.handler.user;

import io.swagger.annotations.Api;
import io.syndesis.dao.manager.DataManager;
import io.syndesis.model.Kind;
import io.syndesis.model.user.User;
import io.syndesis.openshift.OpenShiftService;
import io.syndesis.rest.v1.handler.BaseHandler;
import io.syndesis.rest.v1.operations.Getter;
import io.syndesis.rest.v1.operations.Lister;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.stereotype.Component;
import org.springframework.util.Assert;

import javax.ws.rs.GET;
import javax.ws.rs.Path;
import javax.ws.rs.Produces;
import javax.ws.rs.core.MediaType;
import java.util.Optional;

@Path("/users")
@Api(value = "users")
@Component
public class UserHandler extends BaseHandler implements Lister<User>, Getter<User> {

    private final OpenShiftService openShiftService;

    public UserHandler(DataManager dataMgr, OpenShiftService openShiftService) {
        super(dataMgr);
        this.openShiftService = openShiftService;
    }

    @Override
    public Kind resourceKind() {
        return Kind.User;
    }

    @Path("~")
    @GET
    @Produces(MediaType.APPLICATION_JSON)
    public User whoAmI() {
        String token = String.valueOf(SecurityContextHolder.getContext().getAuthentication().getCredentials());
        io.fabric8.openshift.api.model.User openShiftUser = this.openShiftService.whoAmI(token);
        Assert.notNull(openShiftUser, "A valid user is required");
        return new User.Builder()
            .username(openShiftUser.getMetadata().getName())
            .fullName(Optional.ofNullable(openShiftUser.getFullName()))
            .name(Optional.ofNullable(openShiftUser.getFullName()))
            .build();
    }
}
